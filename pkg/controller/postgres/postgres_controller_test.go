package postgres

import (
	"context"
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/movetokube/postgres-operator/pkg/apis/db/v1alpha1"
	mockpg "github.com/movetokube/postgres-operator/pkg/postgres/mock"
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
		name      = "test-db"
		namespace = "operator"
		sc        *runtime.Scheme
		req       reconcile.Request
		mockCtrl  *gomock.Controller
		pg        *mockpg.MockPG
	)

	BeforeEach(func() {
		// Gomock
		mockCtrl = gomock.NewController(GinkgoT())
		pg = mockpg.NewMockPG(mockCtrl)
		// Create runtime scheme
		sc = scheme.Scheme
		sc.AddKnownTypes(v1alpha1.SchemeGroupVersion, &v1alpha1.Postgres{})
		// Create mock reconcile request
		req = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	It("should not requeue if Postgres does not exist", func() {
		// Create client
		cl := fake.NewFakeClient()
		// Create ReconcilePostgres
		rp := &ReconcilePostgres{
			client: cl,
			scheme: sc,
			pg: pg,
			pgHost: "postgres.local",
		}
		// Call Reconcile
		res, err := rp.Reconcile(req)
		// No error should be returned
		Expect(err).To(BeNil())
		// Request should not be requeued
		Expect(res.Requeue).To(BeFalse())
	})

	Describe("Checking deletion logic", func() {

		var (
			postgresCR *v1alpha1.Postgres
			cl         client.Client
			rp         *ReconcilePostgres
		)

		BeforeEach(func() {
			now := metav1.NewTime(time.Now())
			postgresCR = &v1alpha1.Postgres{
				ObjectMeta: metav1.ObjectMeta{
					Name:              name,
					Namespace:         namespace,
					DeletionTimestamp: &now,
					Finalizers:        []string{"finalizer.db.movetokube.com"},
				},
				Spec: v1alpha1.PostgresSpec{
					Database: name,
				},
				Status: v1alpha1.PostgresStatus{
					Succeeded: true,
					Roles: v1alpha1.PostgresRoles{
						Owner:  name + "-group",
						Reader: name + "-reader",
						Writer: name + "-writer",
					},
				},
			}
		})

		Context("DropOnDelete is unset", func() {

			BeforeEach(func() {
				// Create client
				cl = fake.NewFakeClient([]runtime.Object{postgresCR}...)
				// Create ReconcilePostgres
				rp = &ReconcilePostgres{
					client: cl,
					scheme: sc,
					pg:     pg,
					pgHost: "postgres.local",
				}
			})

			It("should remove finalizer", func() {
				_, err := rp.Reconcile(req)
				// No error should be returned
				Expect(err).To(BeNil())
				// Check updated Postgres
				foundPostgres := &v1alpha1.Postgres{}
				err = cl.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, foundPostgres)
				Expect(err).To(BeNil())
				Expect(len(foundPostgres.GetFinalizers())).To(Equal(0))
			})

		})

		Context("DropOnDelete is enabled", func() {

			var (
				dropGroupRole  *gomock.Call
				dropReaderRole *gomock.Call
				dropWriterRole *gomock.Call
				dropDatabase   *gomock.Call
			)

			BeforeEach(func() {
				// Expected function calls
				pg.EXPECT().GetUser().Return("pguser").AnyTimes()
				dropGroupRole = pg.EXPECT().DropRole(name+"-group", "pguser", name, gomock.Any())
				dropReaderRole = pg.EXPECT().DropRole(name+"-reader", "pguser", name, gomock.Any())
				dropWriterRole = pg.EXPECT().DropRole(name+"-writer", "pguser", name, gomock.Any())
				dropDatabase = pg.EXPECT().DropDatabase(name, gomock.Any())
				// Create client
				postgresCR.Spec.DropOnDelete = true
				cl = fake.NewFakeClient([]runtime.Object{postgresCR}...)
				// Create ReconcilePostgres
				rp = &ReconcilePostgres{
					client: cl,
					scheme: sc,
					pg:     pg,
					pgHost: "postgres.local",
				}
			})

			Context("Deletion is successful", func() {

				It("should remove finalizer", func() {
					// No method should return error
					dropGroupRole.Return(nil)
					dropReaderRole.Return(nil)
					dropWriterRole.Return(nil)
					dropDatabase.Return(nil)
					// Call Reconcile
					_, err := rp.Reconcile(req)
					// No error should be returned
					Expect(err).To(BeNil())
					// Check updated Postgres
					foundPostgres := &v1alpha1.Postgres{}
					err = cl.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, foundPostgres)
					Expect(err).To(BeNil())
					Expect(len(foundPostgres.GetFinalizers())).To(Equal(0))
				})

			})

			Context("Deletion is not successful", func() {

				It("should not remove finalizer when any database action fails", func() {
					// DropDatabase fails
					dropDatabase.Return(fmt.Errorf("Could not drop database"))
					// Call Reconcile
					_, err := rp.Reconcile(req)
					// No error should be returned
					Expect(err).To(Not(BeNil()))
					// Check updated Postgres
					foundPostgres := &v1alpha1.Postgres{}
					err = cl.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, foundPostgres)
					Expect(err).To(BeNil())
					Expect(foundPostgres.GetFinalizers()[0]).To(Equal("finalizer.db.movetokube.com"))
				})

			})

		})

	})

	Describe("Checking creation logic", func() {

		var (
			cl         client.Client
			rp         *ReconcilePostgres
		)
		var postgresCR *v1alpha1.Postgres = &v1alpha1.Postgres{
			ObjectMeta: metav1.ObjectMeta{
				Name:              name,
				Namespace:         namespace,
			},
			Spec: v1alpha1.PostgresSpec{
				Database: name,
			},
		}

		Context("MasterRole is unset", func() {

			BeforeEach(func() {
				// Create client
				cl = fake.NewFakeClient([]runtime.Object{postgresCR}...)
				// Create ReconcilePostgres
				rp = &ReconcilePostgres{
					client: cl,
					scheme: sc,
					pg:     pg,
					pgHost: "postgres.local",
				}
			})

			It("should pick a default role name", func() {
				// CreateGroupRole we're after
				expectedName := name + "-group"
				pg.EXPECT().CreateGroupRole(expectedName).Return(nil)
				// Rest of CreateGroupRole calls (reader and writer)
				pg.EXPECT().CreateGroupRole(gomock.Any()).Return(nil).AnyTimes()
				// CreateDB call
				pg.EXPECT().CreateDB(name, expectedName).Return(nil)
				// Call Reconcile
				rp.Reconcile(req)
			})

		})

		Context("MasterRole is set", func() {

			BeforeEach(func() {
				// Create client
				modPostgres := postgresCR.DeepCopy()
				modPostgres.Spec.MasterRole = "my-master-role"
				cl = fake.NewFakeClient([]runtime.Object{modPostgres}...)
				// Create ReconcilePostgres
				rp = &ReconcilePostgres{
					client: cl,
					scheme: sc,
					pg:     pg,
					pgHost: "postgres.local",
				}
			})

			It("should use name in MasterRole", func() {
				// CreateGroupRole we're after
				expectedName := "my-master-role"
				pg.EXPECT().CreateGroupRole(expectedName).Return(nil)
				// Rest of CreateGroupRole calls (reader and writer)
				pg.EXPECT().CreateGroupRole(gomock.Any()).Return(nil).AnyTimes()
				// CreateDB call
				pg.EXPECT().CreateDB(name, expectedName).Return(nil)
				// Call Reconcile
				rp.Reconcile(req)
			})

		})

		Context("Creation is successful", func() {

			BeforeEach(func() {
				// Create client
				cl = fake.NewFakeClient([]runtime.Object{postgresCR}...)
				// Create ReconcilePostgres
				rp = &ReconcilePostgres{
					client: cl,
					scheme: sc,
					pg:     pg,
					pgHost: "postgres.local",
				}
				// Expected function calls
				pg.EXPECT().CreateGroupRole(gomock.Any()).Return(nil).Times(3)
				pg.EXPECT().CreateDB(name, gomock.Any()).Return(nil)
			})

			It("should update status", func() {
				expectedRoles := v1alpha1.PostgresRoles{
					Owner: name + "-group",
					Reader: name + "-reader",
					Writer: name + "-writer",
				}
				// Call Reconcile
				_, err := rp.Reconcile(req)
				// No error should be returned
				Expect(err).To(BeNil())
				// Check updated Postgres
				foundPostgres := &v1alpha1.Postgres{}
				err = cl.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, foundPostgres)
				Expect(foundPostgres.Status.Roles).To(Equal(expectedRoles))
				Expect(foundPostgres.Status.Succeeded).To(BeTrue())
			})

			It("should set a finalizer", func() {
				expectedFinalizer := "finalizer.db.movetokube.com"
				// Call Reconcile
				_, err := rp.Reconcile(req)
				// No error should be returned
				Expect(err).To(BeNil())
				// Check updated Postgres
				foundPostgres := &v1alpha1.Postgres{}
				err = cl.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, foundPostgres)
				Expect(foundPostgres.GetFinalizers()[0]).To(Equal(expectedFinalizer))
			})

		})

		Context("Creation is not successful", func() {

			BeforeEach(func() {
				// Create client
				cl = fake.NewFakeClient([]runtime.Object{postgresCR}...)
				// Create ReconcilePostgres
				rp = &ReconcilePostgres{
					client: cl,
					scheme: sc,
					pg:     pg,
					pgHost: "postgres.local",
				}
				// Expected function calls
				pg.EXPECT().CreateGroupRole(gomock.Any()).Return(nil).Times(1)
				pg.EXPECT().CreateDB(name, gomock.Any()).Return(fmt.Errorf("Could not create database"))
			})

			It("should not update status", func() {
				expectedRoles := v1alpha1.PostgresRoles{
					Owner: "",
					Reader: "",
					Writer: "",
				}
				// Call Reconcile
				rp.Reconcile(req)
				// Check updated Postgres
				foundPostgres := &v1alpha1.Postgres{}
				cl.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, foundPostgres)
				Expect(foundPostgres.Status.Roles).To(Equal(expectedRoles))
				Expect(foundPostgres.Status.Succeeded).To(BeFalse())
			})

		})

	})

})
