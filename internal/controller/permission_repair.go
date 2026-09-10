package controller

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	dbv1alpha1 "github.com/movetokube/postgres-operator/api/v1alpha1"
	"github.com/movetokube/postgres-operator/pkg/postgres"
	"github.com/robfig/cron/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type repairSchedule struct {
	cron.Schedule
	window, timeout time.Duration
	key             string
}

func parseRepairSchedule(spec *dbv1alpha1.PermissionRepairSpec, now time.Time) (repairSchedule, error) {
	var result repairSchedule
	if len(strings.Fields(spec.Schedule)) != 5 || strings.Contains(spec.Schedule, "=") {
		return result, fmt.Errorf("schedule must have exactly five cron fields")
	}
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	schedule, err := parser.Parse("CRON_TZ=UTC " + spec.Schedule)
	if err != nil {
		return result, fmt.Errorf("invalid cron schedule")
	}
	if schedule.Next(now).IsZero() {
		return result, fmt.Errorf("cron schedule has no next occurrence")
	}
	window := spec.WindowDuration
	if window == "" {
		window = "30m"
	}
	timeout := spec.Timeout
	if timeout == "" {
		timeout = "5m"
	}
	result.window, err = time.ParseDuration(window)
	if err != nil || result.window <= 0 || result.window > 24*time.Hour {
		return result, fmt.Errorf("windowDuration must be positive and at most 24h")
	}
	result.timeout, err = time.ParseDuration(timeout)
	if err != nil || result.timeout <= 0 || result.timeout > result.window {
		return result, fmt.Errorf("timeout must be positive and no longer than windowDuration")
	}
	result.Schedule = schedule
	result.key = fmt.Sprintf("%x", sha256.Sum256([]byte(spec.Schedule+"|UTC|"+window+"|"+timeout)))
	return result, nil
}

func (r *PostgresReconciler) repairNow() time.Time {
	if r.now != nil {
		return r.now().UTC()
	}
	return time.Now().UTC()
}

func (r *PostgresReconciler) reconcilePermissionRepair(ctx context.Context, instance *dbv1alpha1.Postgres) (ctrl.Result, error) {
	before := instance.DeepCopy()
	if instance.Spec.PermissionRepair == nil {
		if instance.Status.PermissionRepair != nil {
			instance.Status.PermissionRepair = nil
			return ctrl.Result{}, r.Status().Patch(ctx, instance, client.MergeFrom(before))
		}
		return ctrl.Result{}, nil
	}
	now := r.repairNow()
	schedule, err := parseRepairSchedule(instance.Spec.PermissionRepair, now)
	if instance.Status.PermissionRepair == nil {
		instance.Status.PermissionRepair = &dbv1alpha1.PermissionRepairStatus{}
	}
	status := instance.Status.PermissionRepair
	if err != nil {
		status.Error = err.Error()
		status.NextRunTime = nil
		status.Configuration = ""
		return ctrl.Result{}, r.Status().Patch(ctx, instance, client.MergeFrom(before))
	}
	// A schema/role change starts a new schedule instead of applying stale work.
	schedule.key = fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%v|%v", schedule.key, instance.Spec.Database, instance.Spec.Schemas, instance.Status.Roles))))
	if status.Configuration != schedule.key || status.NextRunTime == nil {
		next := metav1.NewTime(schedule.Next(now))
		status.NextRunTime = &next
		status.Configuration = schedule.key
		status.Error = ""
		return ctrl.Result{RequeueAfter: next.Sub(now)}, r.Status().Patch(ctx, instance, client.MergeFrom(before))
	}
	due := status.NextRunTime.Time
	if now.Before(due) {
		return ctrl.Result{RequeueAfter: due.Sub(now)}, nil
	}
	next := metav1.NewTime(schedule.Next(now))
	status.NextRunTime = &next
	end := due.Add(schedule.window)
	if !now.Before(end) {
		// Do not catch up outside the maintenance window.
		return ctrl.Result{RequeueAfter: next.Sub(now)}, r.Status().Patch(ctx, instance, client.MergeFrom(before))
	}
	// Persist the occurrence claim before SQL. A restart will not repeat this occurrence.
	attempt := metav1.NewTime(now)
	status.LastAttemptTime = &attempt
	status.Error = "repair interrupted before completion"
	if err := r.Status().Patch(ctx, instance, client.MergeFromWithOptions(before, client.MergeFromWithOptimisticLock{})); err != nil {
		return ctrl.Result{}, err
	}
	before = instance.DeepCopy()
	status = instance.Status.PermissionRepair
	deadline := now.Add(schedule.timeout)
	if end.Before(deadline) {
		deadline = end
	}
	repairCtx, cancel := context.WithTimeout(ctx, deadline.Sub(now))
	defer cancel()
	err = r.pg.RepairPermissions(repairCtx, postgres.PermissionRepair{
		Database: instance.Spec.Database, Schemas: instance.Spec.Schemas,
		Owner: instance.Status.Roles.Owner, Reader: instance.Status.Roles.Reader, Writer: instance.Status.Roles.Writer,
	})
	if err != nil {
		// Connection errors can contain credentials. Store/log a bounded safe diagnostic.
		status.Error = postgres.PermissionRepairError(err)
		log.FromContext(ctx).Info("Permission repair failed", "reason", status.Error)
	} else {
		success := metav1.NewTime(r.repairNow())
		status.LastSuccessTime = &success
		status.Error = ""
	}
	if err := r.Status().Patch(ctx, instance, client.MergeFromWithOptions(before, client.MergeFromWithOptimisticLock{})); err != nil {
		return ctrl.Result{}, err
	}
	delay := status.NextRunTime.Sub(r.repairNow())
	if delay <= 0 {
		delay = time.Second
	}
	return ctrl.Result{RequeueAfter: delay}, nil
}
