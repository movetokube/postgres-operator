package postgres

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	"github.com/movetokube/postgres-operator/pkg/apis/db/v1alpha1"
	mockpg "github.com/movetokube/postgres-operator/pkg/postgres/mock"
	"github.com/movetokube/postgres-operator/pkg/utils"
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
		sc.AddKnownTypes(v1alpha1.SchemeGroupVersion, &v1alpha1.PostgresList{})
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
			pg:     pg,
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
					Finalizers:        []string{"foregroundDeletion"},
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

			It("should not try to delete roles or database", func() {
				// Neither DropRole nor DropDatabase should be called
				pg.EXPECT().DropRole(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				pg.EXPECT().DropDatabase(gomock.Any(), gomock.Any()).Times(0)
				// Call Reconcile
				rp.Reconcile(req)
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
				// Create Postgres with DropOnDelete == true
				dropPostgres := postgresCR.DeepCopy()
				dropPostgres.Spec.DropOnDelete = true
				// Create client
				cl = fake.NewFakeClient([]runtime.Object{dropPostgres}...)
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
					Expect(foundPostgres.GetFinalizers()[0]).To(Equal("foregroundDeletion"))
				})

			})

			Context("Another Postgres exists with same database", func() {

				BeforeEach(func() {
					// Create two Postgres with same database name
					dropPostgres := postgresCR.DeepCopy()
					dropPostgres.Spec.DropOnDelete = true
					anotherPostgres := postgresCR.DeepCopy()
					anotherPostgres.Namespace = "default"
					// Create client
					cl = fake.NewFakeClient([]runtime.Object{dropPostgres, anotherPostgres}...)
					// Create ReconcilePostgres
					rp = &ReconcilePostgres{
						client: cl,
						scheme: sc,
						pg:     pg,
						pgHost: "postgres.local",
					}
				})

				It("should not drop roles or database", func() {
					// Expect no method calls
					dropDatabase.Times(0)
					dropGroupRole.Times(0)
					dropReaderRole.Times(0)
					dropWriterRole.Times(0)
					// Call Reconcile
					rp.Reconcile(req)
				})

			})

		})

	})

	Describe("Checking creation logic", func() {

		var (
			cl client.Client
			rp *ReconcilePostgres
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

		Context("Correct annotation filter is set", func() {

			BeforeEach(func() {
				// Create client
				modPostgres := postgresCR.DeepCopy()
				modPostgres.Annotations = map[string]string{
					utils.INSTANCE_ANNOTATION: "my-instance",
				}
				cl = fake.NewFakeClient([]runtime.Object{modPostgres}...)
				// Create ReconcilePostgres
				rp = &ReconcilePostgres{
					client:         cl,
					scheme:         sc,
					pg:             pg,
					pgHost:         "postgres.local",
					instanceFilter: "my-instance",
				}
			})

			It("should create the database", func() {
				pg.EXPECT().CreateGroupRole(gomock.Any()).Return(nil).Times(3)
				pg.EXPECT().CreateDB(name, gomock.Any()).Return(nil)
				// Call Reconcile
				rp.Reconcile(req)
			})
		})

		Context("Incorrect annotation filter is set", func() {

			BeforeEach(func() {
				// Create client
				modPostgres := postgresCR.DeepCopy()
				modPostgres.Annotations = map[string]string{
					utils.INSTANCE_ANNOTATION: "my-instance",
				}
				cl = fake.NewFakeClient([]runtime.Object{modPostgres}...)
				// Create ReconcilePostgres
				rp = &ReconcilePostgres{
					client:         cl,
					scheme:         sc,
					pg:             pg,
					pgHost:         "postgres.local",
					instanceFilter: "my-other-instance",
				}
			})

			It("should create the database", func() {
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
					Owner:  name + "-group",
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
				expectedFinalizer := "foregroundDeletion"
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
					Owner:  "",
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

	Describe("Checking extensions logic", func() {

		var (
			cl client.Client
			rp *ReconcilePostgres
		)
		var postgresCR *v1alpha1.Postgres = &v1alpha1.Postgres{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: v1alpha1.PostgresSpec{
				Database: name,
			},
			Status: v1alpha1.PostgresStatus{
				// So it doesn't run creation logic
				Succeeded: true,
			},
		}

		Context("Postgres has no extensions", func() {

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

			It("should not try to create extensions", func() {
				// CreateExtension should not be called
				pg.EXPECT().CreateExtension(name, gomock.Any(), gomock.Any()).Times(0)
				// Call Reconcile
				rp.Reconcile(req)
			})

			It("should not set status", func() {
				// Call reconcile
				rp.Reconcile(req)
				// Check updated Postgres
				foundPostgres := &v1alpha1.Postgres{}
				cl.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, foundPostgres)
				Expect(len(foundPostgres.Status.Extensions)).To(Equal(0))
			})

		})

		Context("Postgres has extensions", func() {

			BeforeEach(func() {
				// Add extensions to Postgres object
				extPostgres := postgresCR.DeepCopy()
				extPostgres.Spec.Extensions = []string{"pg_stat_statements", "hstore"}
				// Create client
				cl = fake.NewFakeClient([]runtime.Object{extPostgres}...)
				// Create ReconcilePostgres
				rp = &ReconcilePostgres{
					client: cl,
					scheme: sc,
					pg:     pg,
					pgHost: "postgres.local",
				}
			})

			Context("Creation is successful", func() {

				BeforeEach(func() {
					// Expected method calls
					pg.EXPECT().CreateExtension(name, "pg_stat_statements", gomock.Any()).Return(nil).Times(1)
					pg.EXPECT().CreateExtension(name, "hstore", gomock.Any()).Return(nil).Times(1)
				})

				It("should update status", func() {
					// Call reconcile
					rp.Reconcile(req)
					// Check updated Postgres
					foundPostgres := &v1alpha1.Postgres{}
					cl.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, foundPostgres)
					Expect(len(foundPostgres.Status.Extensions)).To(Equal(2))
					Expect(foundPostgres.Status.Extensions[0]).To(Equal("pg_stat_statements"))
					Expect(foundPostgres.Status.Extensions[1]).To(Equal("hstore"))
				})

			})

			Context("Creation is not successful", func() {

				BeforeEach(func() {
					// Expected method calls
					pg.EXPECT().CreateExtension(name, "pg_stat_statements", gomock.Any()).Return(fmt.Errorf("Could not create extension")).Times(1)
					pg.EXPECT().CreateExtension(name, "hstore", gomock.Any()).Return(nil).Times(1)
				})

				It("should update status", func() {
					// Call reconcile
					rp.Reconcile(req)
					// Check updated Postgres
					foundPostgres := &v1alpha1.Postgres{}
					cl.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, foundPostgres)
					Expect(len(foundPostgres.Status.Extensions)).To(Equal(1))
					Expect(foundPostgres.Status.Extensions[0]).To(Equal("hstore"))
				})

			})

		})

		Context("Subset of extensions already created", func() {

			BeforeEach(func() {
				// Add extensions to Postgres object
				extPostgres := postgresCR.DeepCopy()
				extPostgres.Spec.Extensions = []string{"pg_stat_statements", "hstore"}
				extPostgres.Status.Extensions = []string{"hstore"}
				// Create client
				cl = fake.NewFakeClient([]runtime.Object{extPostgres}...)
				// Create ReconcilePostgres
				rp = &ReconcilePostgres{
					client: cl,
					scheme: sc,
					pg:     pg,
					pgHost: "postgres.local",
				}
			})

			Context("Creation is successful", func() {

				It("should not recreate extisting extension", func() {
					// Expected method calls
					pg.EXPECT().CreateExtension(name, "pg_stat_statements", gomock.Any()).Return(nil).Times(1)
					pg.EXPECT().CreateExtension(name, "hstore", gomock.Any()).Times(0)
					// Call reconcile
					rp.Reconcile(req)
					// Check updated Postgres
					foundPostgres := &v1alpha1.Postgres{}
					cl.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, foundPostgres)
					Expect(len(foundPostgres.Status.Extensions)).To(Equal(2))
					Expect(foundPostgres.Status.Extensions[0]).To(Equal("hstore"))
					Expect(foundPostgres.Status.Extensions[1]).To(Equal("pg_stat_statements"))
				})

			})

		})

	})

	Describe("Checking schemas logic", func() {

		var (
			cl client.Client
			rp *ReconcilePostgres
		)
		var postgresCR *v1alpha1.Postgres = &v1alpha1.Postgres{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: v1alpha1.PostgresSpec{
				Database: name,
			},
			Status: v1alpha1.PostgresStatus{
				// So it doesn't run creation logic
				Succeeded: true,
				Roles: v1alpha1.PostgresRoles{
					Owner:  name + "-group",
					Reader: name + "-reader",
					Writer: name + "-writer",
				},
			},
		}

		Context("Postgres has no schemas", func() {

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

			It("should not try to create schemas", func() {
				// CreateSchema should not be called
				pg.EXPECT().CreateSchema(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				// Call Reconcile
				rp.Reconcile(req)
			})

			It("should not set status", func() {
				// Call reconcile
				rp.Reconcile(req)
				// Check updated Postgres
				foundPostgres := &v1alpha1.Postgres{}
				cl.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, foundPostgres)
				Expect(len(foundPostgres.Status.Schemas)).To(Equal(0))
			})

		})

		Context("Postgres has schemas", func() {

			BeforeEach(func() {
				// Add schemas to Postgres object
				schemaPostgres := postgresCR.DeepCopy()
				schemaPostgres.Spec.Schemas = []string{"customers", "stores"}
				// Create client
				cl = fake.NewFakeClient([]runtime.Object{schemaPostgres}...)
				// Create ReconcilePostgres
				rp = &ReconcilePostgres{
					client: cl,
					scheme: sc,
					pg:     pg,
					pgHost: "postgres.local",
				}
			})

			Context("Creation is successful", func() {

				BeforeEach(func() {
					// Expected method calls
					// customers schema
					pg.EXPECT().CreateSchema(name, name+"-group", "customers", gomock.Any()).Return(nil).Times(1)
					pg.EXPECT().SetSchemaPrivileges(name, name+"-group", gomock.Any(), "customers", gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(3)
					// stores schema
					pg.EXPECT().CreateSchema(name, name+"-group", "stores", gomock.Any()).Return(nil).Times(1)
					pg.EXPECT().SetSchemaPrivileges(name, name+"-group", gomock.Any(), "stores", gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(3)
				})

				It("should update status", func() {
					// Call reconcile
					rp.Reconcile(req)
					// Check updated Postgres
					foundPostgres := &v1alpha1.Postgres{}
					cl.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, foundPostgres)
					Expect(len(foundPostgres.Status.Schemas)).To(Equal(2))
					Expect(foundPostgres.Status.Schemas[0]).To(Equal("customers"))
					Expect(foundPostgres.Status.Schemas[1]).To(Equal("stores"))
				})

			})

			Context("Creation is not successful", func() {

				BeforeEach(func() {
					// Expected method calls
					// customers schema errors
					pg.EXPECT().CreateSchema(name, name+"-group", "customers", gomock.Any()).Return(fmt.Errorf("Could not create schema")).Times(1)
					pg.EXPECT().SetSchemaPrivileges(name, name+"-group", gomock.Any(), "customers", gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(0)
					// stores schema
					pg.EXPECT().CreateSchema(name, name+"-group", "stores", gomock.Any()).Return(nil).Times(1)
					pg.EXPECT().SetSchemaPrivileges(name, name+"-group", name+"-reader", "stores", gomock.Any(), false, gomock.Any()).Return(nil).Times(1)
					pg.EXPECT().SetSchemaPrivileges(name, name+"-group", name+"-writer", "stores", gomock.Any(), true, gomock.Any()).Return(nil).Times(1)
					pg.EXPECT().SetSchemaPrivileges(name, name+"-group", name+"-group", "stores", gomock.Any(), true, gomock.Any()).Return(nil).Times(1)
				})

				It("should update status", func() {
					// Call reconcile
					rp.Reconcile(req)
					// Check updated Postgres
					foundPostgres := &v1alpha1.Postgres{}
					cl.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, foundPostgres)
					Expect(len(foundPostgres.Status.Schemas)).To(Equal(1))
					Expect(foundPostgres.Status.Schemas[0]).To(Equal("stores"))
				})

			})

		})

		Context("Subset of schema already created", func() {

			BeforeEach(func() {
				// Add schemas to Postgres object
				schemaPostgres := postgresCR.DeepCopy()
				schemaPostgres.Spec.Schemas = []string{"customers", "stores"}
				schemaPostgres.Status.Schemas = []string{"stores"}
				// Create client
				cl = fake.NewFakeClient([]runtime.Object{schemaPostgres}...)
				// Create ReconcilePostgres
				rp = &ReconcilePostgres{
					client: cl,
					scheme: sc,
					pg:     pg,
					pgHost: "postgres.local",
				}
			})

			Context("Creation is successful", func() {

				It("should not recreate extisting schema", func() {
					// Expected method calls
					// customers schema
					pg.EXPECT().CreateSchema(name, name+"-group", "customers", gomock.Any()).Return(nil).Times(1)
					pg.EXPECT().SetSchemaPrivileges(name, name+"-group", gomock.Any(), "customers", gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(3)
					// stores schema already exists
					pg.EXPECT().CreateSchema(name, name+"-group", "stores", gomock.Any()).Times(0)
					// Call reconcile
					rp.Reconcile(req)
					// Check updated Postgres
					foundPostgres := &v1alpha1.Postgres{}
					cl.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, foundPostgres)
					Expect(len(foundPostgres.Status.Schemas)).To(Equal(2))
					Expect(foundPostgres.Status.Schemas[0]).To(Equal("stores"))
					Expect(foundPostgres.Status.Schemas[1]).To(Equal("customers"))
				})

			})

		})

	})

})
