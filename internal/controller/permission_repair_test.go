package controller

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	db "github.com/movetokube/postgres-operator/api/v1alpha1"
	"github.com/movetokube/postgres-operator/pkg/postgres"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type repairSpy struct {
	postgres.PG
	calls   int
	request postgres.PermissionRepair
	err     error
	timeout time.Duration
}

func (s *repairSpy) RepairPermissions(ctx context.Context, p postgres.PermissionRepair) error {
	s.calls++
	s.request = p
	if deadline, ok := ctx.Deadline(); ok {
		s.timeout = time.Until(deadline)
	}
	return s.err
}
func repairFixture(t *testing.T) (*PostgresReconciler, *db.Postgres, *repairSpy, *time.Time) {
	t.Helper()
	now := time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC)
	instance := &db.Postgres{ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "test", Finalizers: []string{"finalizer.db.movetokube.com"}},
		Spec:   db.PostgresSpec{Database: "app-db", MasterRole: "app-owner", Schemas: []string{"public", "billing"}, PermissionRepair: &db.PermissionRepairSpec{Schedule: "0 2 * * *"}},
		Status: db.PostgresStatus{Succeeded: true, Schemas: []string{"public", "billing"}, Roles: db.PostgresRoles{Owner: "app-owner", Reader: "app-reader", Writer: "app-writer"}}}
	scheme := runtime.NewScheme()
	if err := db.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	spy := &repairSpy{}
	r := &PostgresReconciler{Client: fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(instance).WithObjects(instance).Build(), pg: spy, now: func() time.Time { return now }}
	return r, instance, spy, &now
}
func runRepair(t *testing.T, r *PostgresReconciler, p *db.Postgres) ctrl.Result {
	t.Helper()
	result, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: p.Name, Namespace: p.Namespace}})
	if err != nil {
		t.Fatal(err)
	}
	if err = r.Get(context.Background(), types.NamespacedName{Name: p.Name, Namespace: p.Namespace}, p); err != nil {
		t.Fatal(err)
	}
	return result
}
func TestPermissionRepairSchedule(t *testing.T) {
	now := time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		spec db.PermissionRepairSpec
		want string
		bad  bool
	}{
		{"UTC", db.PermissionRepairSpec{Schedule: "0 2 * * *"}, "2026-09-09T02:00:00Z", false},
		{"steps", db.PermissionRepairSpec{Schedule: "*/15 * * * *"}, "2026-09-09T00:15:00Z", false},
		{"six fields", db.PermissionRepairSpec{Schedule: "0 0 2 * * *"}, "", true},
		{"descriptor", db.PermissionRepairSpec{Schedule: "@daily"}, "", true},
		{"range", db.PermissionRepairSpec{Schedule: "65 2 * * *"}, "", true},
		{"impossible", db.PermissionRepairSpec{Schedule: "0 2 31 2 *"}, "", true},
		{"timeout", db.PermissionRepairSpec{Schedule: "0 2 * * *", Timeout: "0s"}, "", true},
		{"oversized timeout", db.PermissionRepairSpec{Schedule: "0 2 * * *", Timeout: "31m"}, "", true},
		{"window", db.PermissionRepairSpec{Schedule: "0 2 * * *", WindowDuration: "48h"}, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := parseRepairSchedule(&tt.spec, now)
			if (err != nil) != tt.bad {
				t.Fatalf("error=%v", err)
			}
			if err == nil && s.Next(now).UTC().Format(time.RFC3339) != tt.want {
				t.Fatalf("next=%v", s.Next(now))
			}
		})
	}

}
func TestPermissionRepairLifecycle(t *testing.T) {
	r, p, spy, now := repairFixture(t)
	result := runRepair(t, r, p)
	if spy.calls != 0 || result.RequeueAfter != 2*time.Hour {
		t.Fatalf("initial: calls=%d result=%v", spy.calls, result)
	}
	// A fresh reconciler uses persisted state, not an in-memory cron job.
	r = &PostgresReconciler{Client: r.Client, pg: spy, now: r.now}
	*now = now.Add(2*time.Hour + time.Minute)
	runRepair(t, r, p)
	status := p.Status.PermissionRepair
	if spy.calls != 1 || status.LastSuccessTime == nil || status.Error != "" || !p.Status.Succeeded {
		t.Fatalf("status=%+v calls=%d", status, spy.calls)
	}
	if spy.request.Owner != "app-owner" || len(spy.request.Schemas) != 2 || spy.timeout <= 0 || spy.timeout > 5*time.Minute {
		t.Fatalf("request=%+v timeout=%v", spy.request, spy.timeout)
	}
	runRepair(t, r, p)
	if spy.calls != 1 {
		t.Fatal("status event repeated SQL")
	}
	// Failure preserves last success and provisioning state; retry is next cron occurrence.
	success := status.LastSuccessTime.DeepCopy()
	*now = status.NextRunTime.Time
	spy.err = errors.New("postgresql://user:secret@host/db")
	runRepair(t, r, p)
	if spy.calls != 2 || !p.Status.Succeeded || !p.Status.PermissionRepair.LastSuccessTime.Equal(success) || strings.Contains(p.Status.PermissionRepair.Error, "secret") || p.Status.PermissionRepair.Error == "" {
		t.Fatalf("failure status=%+v", p.Status)
	}
	runRepair(t, r, p)
	if spy.calls != 2 {
		t.Fatal("failure retried outside schedule")
	}
}
func TestPermissionRepairMissedWindowAndChanges(t *testing.T) {
	r, p, spy, now := repairFixture(t)
	runRepair(t, r, p)
	*now = now.Add(3 * time.Hour)
	runRepair(t, r, p)
	if spy.calls != 0 || p.Status.PermissionRepair.NextRunTime.Day() != 10 {
		t.Fatal("missed window executed")
	}
	p.Spec.PermissionRepair.Schedule = "0 4 * * *"
	if err := r.Update(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	runRepair(t, r, p)
	if p.Status.PermissionRepair.NextRunTime.UTC().Hour() != 4 {
		t.Fatalf("schedule change not applied: spec=%+v status=%+v", p.Spec.PermissionRepair, p.Status.PermissionRepair)
	}
	p.Spec.PermissionRepair.Schedule = "invalid"
	if err := r.Update(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	result := runRepair(t, r, p)
	if result.RequeueAfter != 0 || p.Status.PermissionRepair.Error == "" || spy.calls != 0 {
		t.Fatal("invalid config not rejected")
	}
	// Removing schedule clears state (helper avoids intentional legacy schema grants).
	p.Spec.PermissionRepair = nil
	if _, err := r.reconcilePermissionRepair(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	if p.Status.PermissionRepair != nil {
		t.Fatal("disabled schedule retained")
	}
}
func TestPermissionRepairDeadlineAtWindowEnd(t *testing.T) {
	r, p, spy, now := repairFixture(t)
	runRepair(t, r, p)
	*now = now.Add(2*time.Hour + 29*time.Minute)
	runRepair(t, r, p)
	if spy.timeout <= 0 || spy.timeout > time.Minute {
		t.Fatalf("timeout=%v", spy.timeout)
	}
}

func TestPermissionRepairClaimConflict(t *testing.T) {
	r, p, spy, now := repairFixture(t)
	runRepair(t, r, p)
	stale := p.DeepCopy()
	*now = now.Add(2 * time.Hour)
	if _, err := r.reconcilePermissionRepair(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	if _, err := r.reconcilePermissionRepair(context.Background(), stale); err == nil {
		t.Fatal("stale claim should conflict")
	}
	if spy.calls != 1 {
		t.Fatal("conflicting claim executed SQL")
	}
}
func TestPermissionRepairExcludedResources(t *testing.T) {
	t.Run("other instance", func(t *testing.T) {
		r, p, spy, _ := repairFixture(t)
		r.instanceFilter = "other"
		runRepair(t, r, p)
		if spy.calls != 0 || p.Status.PermissionRepair != nil {
			t.Fatal("wrong instance scheduled")
		}
	})
	t.Run("deleted", func(t *testing.T) {
		r, p, spy, _ := repairFixture(t)
		if err := r.Delete(context.Background(), p); err != nil {
			t.Fatal(err)
		}
		_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: p.Name, Namespace: p.Namespace}})
		if err != nil {
			t.Fatal(err)
		}
		if spy.calls != 0 {
			t.Fatal("deleted resource repaired")
		}
	})
}

// A host-local timezone or daylight-saving boundary must never move the UTC window.
func TestPermissionRepairAlwaysUTC(t *testing.T) {
	previousLocal := time.Local
	time.Local = time.FixedZone("operator-local", -7*60*60)
	t.Cleanup(func() { time.Local = previousLocal })
	spec := &db.PermissionRepairSpec{Schedule: "0 2 * * *"}
	for _, day := range []string{"2026-03-28", "2026-03-29", "2026-10-24", "2026-10-25"} {
		t.Run(day, func(t *testing.T) {
			midnight, err := time.Parse("2006-01-02", day)
			if err != nil {
				t.Fatal(err)
			}
			now := midnight.In(time.Local)
			schedule, err := parseRepairSchedule(spec, now)
			if err != nil {
				t.Fatal(err)
			}
			if got := schedule.Next(now); !got.Equal(midnight.Add(2 * time.Hour)) {
				t.Fatalf("next=%v, expected 02:00 UTC", got)
			}
			r, p, spy, clock := repairFixture(t)
			*clock = now
			if r.repairNow().Location() != time.UTC {
				t.Fatal("controller clock is not UTC")
			}
			result := runRepair(t, r, p)
			if result.RequeueAfter != 2*time.Hour || !p.Status.PermissionRepair.NextRunTime.Equal(&metav1.Time{Time: midnight.Add(2 * time.Hour)}) || spy.calls != 0 {
				t.Fatalf("wrong UTC scheduling: %+v", p.Status.PermissionRepair)
			}
			*clock = now.Add(2 * time.Hour)
			runRepair(t, r, p)
			if spy.calls != 1 {
				t.Fatal("repair did not run at 02:00 UTC")
			}
		})
	}
}
