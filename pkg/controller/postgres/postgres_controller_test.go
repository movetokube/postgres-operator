package postgres

import (
	"context"
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"time"

	"github.com/go-logr/logr"
	"github.com/movetokube/postgres-operator/pkg/apis/db/v1alpha1"
	fakepg "github.com/movetokube/postgres-operator/pkg/postgres/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("ReconcilePostgres", func() {
	var (
		sc *runtime.Scheme
		pg *fakepg.MockPostgres
	)

	BeforeEach(func() {
		// Create runtime scheme
		sc = scheme.Scheme
		sc.AddKnownTypes(v1alpha1.SchemeGroupVersion, &v1alpha1.Postgres{})
		// Create MockPostgres
		pg = fakepg.NewMockPostgres()
	})

	Describe("Checking if Postgres exists", func() {
		Context("It does not exist", func() {
			It("should not requeue request", func() {
				// Create a fake client to mock API calls
				cl := fake.NewFakeClient()

				// Create ReconcileNetworkPolicy
				r := &ReconcilePostgres{
					client: cl,
					scheme: sc,
					pg:     pg,
					pgHost: "postgres.local",
				}

				// Mock request to simulate Reconcile() being called on an event for a watched resource
				req := reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      "test-db",
						Namespace: "operator",
					},
				}
				res, err := r.Reconcile(req)
				// No error should be returned
				Expect(err).To(BeNil())
				// Request should not be requeued
				Expect(res.Requeue).To(BeFalse())
			})
		})
	})

	Describe("Checking if status is being updated", func() {
		var (
			cl        client.Client
			rp        *ReconcilePostgres
			req       reconcile.Request
			namespace = "operator"
			name      = "test-db"
		)
		var postgresCR *v1alpha1.Postgres = &v1alpha1.Postgres{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: v1alpha1.PostgresSpec{
				Database: name,
			},
		}

		BeforeEach(func() {
			// Create a fake client to mock API calls
			objs := []runtime.Object{postgresCR}
			cl = fake.NewFakeClient(objs...)
			// Create ReconcilePostgres
			rp = &ReconcilePostgres{
				client: cl,
				scheme: sc,
				pg:     pg,
				pgHost: "postgres.local",
			}
			// Mock request to simulate Reconcile() being called on an event for a watched resource
			req = reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      name,
					Namespace: namespace,
				},
			}
		})

		Describe("Checking owner role", func() {
			Context("No MasterRole set", func() {
				It("should set a default group role", func() {
					expected := name + "-group"

					// Call Reconcile
					_, err := rp.Reconcile(req)
					// No error should be returned
					Expect(err).To(BeNil())
					// Status should be updated with generated group name
					foundPostgres := &v1alpha1.Postgres{}
					err = cl.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, foundPostgres)
					Expect(err).To(BeNil())
					Expect(foundPostgres.Status.Roles.Owner).To(Equal(expected))
				})
			})

			Context("MasterRole set", func() {
				It("should use MasterRole as group role", func() {
					expected := "my-owner-role"
					// Set MasterRole
					postgresCR.Spec.MasterRole = expected
					// Create a fake client to mock API calls
					objs := []runtime.Object{postgresCR}
					cl := fake.NewFakeClient(objs...)

					// Create ReconcilePostgres
					r := &ReconcilePostgres{
						client: cl,
						scheme: sc,
						pg:     pg,
						pgHost: "postgres.local",
					}

					// Call Reconcile
					_, err := r.Reconcile(req)
					// No error should be returned
					Expect(err).To(BeNil())
					// Status should be updated with generated group name
					foundPostgres := &v1alpha1.Postgres{}
					err = cl.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, foundPostgres)
					Expect(err).To(BeNil())
					Expect(foundPostgres.Status.Roles.Owner).To(Equal(expected))
				})
			})
		})

		Describe("Checking reader and writer roles", func() {
			It("should have set reader and writer role", func() {
				expectedReader := name + "-reader"
				expectedWriter := name + "-writer"

				// Call Reconcile
				_, err := rp.Reconcile(req)
				// No error should be returned
				Expect(err).To(BeNil())
				// Status should be updated with generated reader and writer groups
				foundPostgres := &v1alpha1.Postgres{}
				err = cl.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, foundPostgres)
				Expect(err).To(BeNil())
				Expect(foundPostgres.Status.Roles.Reader).To(Equal(expectedReader))
				Expect(foundPostgres.Status.Roles.Writer).To(Equal(expectedWriter))
			})
		})

		Describe("Checking succeeded status", func() {
			Context("Database creation is successful", func() {
				It("should set succeeded as true", func() {
					// Call Reconcile
					_, err := rp.Reconcile(req)
					// No error should be returned
					Expect(err).To(BeNil())
					// Status should be updated with database name
					foundPostgres := &v1alpha1.Postgres{}
					err = cl.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, foundPostgres)
					Expect(err).To(BeNil())
					Expect(foundPostgres.Status.Succeeded).To(BeTrue())
				})
			})

			Context("Database creation fails", func() {
				It("should set succeeded as false", func() {
					// Mock CreateDB to return error
					pg.MockCreateDB = func(dbname, username string) error {
						return fmt.Errorf("There was an error!")
					}

					// Call Reconcile
					_, err := rp.Reconcile(req)
					// An error should be returned
					Expect(err).To(Not(BeNil()))
					// Status should be updated with database name
					foundPostgres := &v1alpha1.Postgres{}
					err = cl.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, foundPostgres)
					Expect(err).To(BeNil())
					Expect(foundPostgres.Status.Succeeded).To(BeFalse())
				})
			})
		})
	})

	Describe("Checking deletion logic", func() {
		var (
			cl                client.Client
			rp                *ReconcilePostgres
			req               reconcile.Request
			postgresCR        *v1alpha1.Postgres
			dropRoleCalls     int
			dropDatabaseCalls int
			namespace         = "operator"
			name              = "test-db"
		)

		BeforeEach(func() {
			// Create a Postgres object
			now := metav1.NewTime(time.Now())
			postgresCR = &v1alpha1.Postgres{
				ObjectMeta: metav1.ObjectMeta{
					Name:              name,
					Namespace:         namespace,
					DeletionTimestamp: &now,
				},
				Spec: v1alpha1.PostgresSpec{
					Database: name,
				},
				Status: v1alpha1.PostgresStatus{
					Succeeded: true,
				},
			}
			// Mock postgres
			pg = fakepg.NewMockPostgres()
			pg.MockDropRole = func(role, newOwner, database string, logger logr.Logger) error {
				dropRoleCalls += 1
				return nil
			}
			pg.MockDropDatabase = func(database string, logger logr.Logger) error {
				dropDatabaseCalls += 1
				return nil
			}

			// Create a fake client to mock API calls
			objs := []runtime.Object{postgresCR}
			cl = fake.NewFakeClient(objs...)
			// Create ReconcilePostgres
			rp = &ReconcilePostgres{
				client: cl,
				scheme: sc,
				pg:     pg,
				pgHost: "postgres.local",
			}
			// Mock request to simulate Reconcile() being called on an event for a watched resource
			req = reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      name,
					Namespace: namespace,
				},
			}
			// Reset call counters
			dropRoleCalls = 0
			dropDatabaseCalls = 0
		})

		Context("DropOnDelete is unset", func() {
			It("should not drop database and roles", func() {
				var (
					expectedDropRole     = 0
					expectedDropDatabase = 0
				)

				// Call Reconcile
				_, err := rp.Reconcile(req)
				// No error should be returned
				Expect(err).To(BeNil())
				// Checking how many times funcions were called
				Expect(dropRoleCalls).To(Equal(expectedDropRole))
				Expect(dropDatabaseCalls).To(Equal(expectedDropDatabase))
			})
		})

		Context("DropOnDelete is set to true", func() {
			It("should drop database and roles", func() {
				var (
					expectedDropRole     = 3
					expectedDropDatabase = 1
				)

				postgresCR.Spec.DropOnDelete = true
				// Create a fake client to mock API calls
				objs := []runtime.Object{postgresCR}
				cl = fake.NewFakeClient(objs...)
				// Create ReconcilePostgres
				rp = &ReconcilePostgres{
					client: cl,
					scheme: sc,
					pg:     pg,
					pgHost: "postgres.local",
				}

				// Call Reconcile
				_, err := rp.Reconcile(req)
				// No error should be returned
				Expect(err).To(BeNil())
				// Checking how many times funcions were called
				Expect(dropRoleCalls).To(Equal(expectedDropRole))
				Expect(dropDatabaseCalls).To(Equal(expectedDropDatabase))
			})
		})
	})
})
