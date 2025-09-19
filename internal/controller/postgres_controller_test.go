package controller

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/movetokube/postgres-operator/api/v1alpha1"
	mockpg "github.com/movetokube/postgres-operator/pkg/postgres/mock"
	"github.com/movetokube/postgres-operator/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("PostgresReconciler", func() {
	const (
		name      = "test-db"
		namespace = "operator"
	)
	var (
		sc       *runtime.Scheme
		req      reconcile.Request
		mockCtrl *gomock.Controller
		pg       *mockpg.MockPG
		rp       *PostgresReconciler
		cl       client.Client
	)

	initClient := func(pg *v1alpha1.Postgres, markAsDeleted bool) {
		statusCopy := pg.Status.DeepCopy()
		if markAsDeleted {
			pg.SetFinalizers([]string{"finalizer.db.movetokube.com"})
		}
		Expect(cl.Create(ctx, pg)).To(BeNil())
		statusCopy.DeepCopyInto(&pg.Status)
		// create status separately, because it is a subresource that is
		// omitted and zeroed by default
		Expect(cl.Status().Update(ctx, pg)).To(BeNil())
		if markAsDeleted {
			Expect(cl.Delete(ctx, pg, &client.DeleteOptions{GracePeriodSeconds: new(int64)})).To(BeNil())
		}
	}

	listPGs := func() {
		l := v1alpha1.PostgresList{}
		Expect(cl.List(ctx, &l)).NotTo(HaveOccurred())

		for i, el := range l.Items {
			GinkgoWriter.Println(i, el)
		}
	}

	runReconcile := func(rp *PostgresReconciler, ctx context.Context, req reconcile.Request) (err error) {
		_, err = rp.Reconcile(ctx, req)
		if k8sManager != nil {
			k8sManager.GetCache().WaitForCacheSync(ctx)
		}
		return err
	}

	BeforeEach(func() {
		// Gomock
		mockCtrl = gomock.NewController(GinkgoT())
		pg = mockpg.NewMockPG(mockCtrl)
		pg.EXPECT().AlterDatabaseOwner(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
		pg.EXPECT().ReassignDatabaseOwner(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
		pg.EXPECT().ReassignDatabaseOwner(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
		cl = k8sClient
		// Create runtime scheme
		sc = scheme.Scheme
		sc.AddKnownTypes(v1alpha1.GroupVersion, &v1alpha1.Postgres{})
		sc.AddKnownTypes(v1alpha1.GroupVersion, &v1alpha1.PostgresList{})
		// Create PostgresReconciler
		rp = &PostgresReconciler{
			Client: managerClient,
			Scheme: sc,
			pg:     pg,
		}
		if k8sManager != nil {
			rp.SetupWithManager(k8sManager)
		}
		// Create mock reconcile request
		req = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}
	})

	AfterEach(func() {
		Expect(clearPgs(namespace)).To(BeNil())
		if k8sManager != nil {
			k8sManager.GetCache().WaitForCacheSync(ctx)
		}
		mockCtrl.Finish()
	})

	It("should have a working client", func() {
		pg := &v1alpha1.Postgres{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "clienttest",
				Namespace: namespace,
			},
			Spec:   v1alpha1.PostgresSpec{Database: "clienttest"},
			Status: v1alpha1.PostgresStatus{Succeeded: true},
		}
		pg2 := pg.DeepCopy()
		pg2.Name = "clienttest2"

		initClient(pg, false)
		initClient(pg2, true)

		instance := &v1alpha1.Postgres{}
		Expect(managerClient.Get(ctx, types.NamespacedName{Name: "clienttest", Namespace: namespace}, instance)).To(BeNil())
		Expect(instance.ObjectMeta.Name).To(Equal("clienttest"))
		Expect(instance.Status.Succeeded).To(BeTrue())

		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "clienttest2", Namespace: namespace}, instance)).NotTo(HaveOccurred())
		Expect(instance.ObjectMeta.Name).To(Equal("clienttest2"))

		Expect(clearPgs(namespace)).NotTo(HaveOccurred())
		l := v1alpha1.PostgresList{}
		Expect(cl.List(ctx, &l)).NotTo(HaveOccurred())
		listPGs()
		Expect(l.Items).To(BeEmpty())
	})

	It("should not requeue if Postgres does not exist", func() {
		// Call Reconcile
		res, err := rp.Reconcile(ctx, req)
		// No error should be returned
		Expect(err).NotTo(HaveOccurred())
		// Request should not be requeued
		Expect(res.Requeue).To(BeFalse())
	})

	Describe("Checking deletion logic", func() {

		var (
			postgresCR *v1alpha1.Postgres
		)

		BeforeEach(func() {
			postgresCR = &v1alpha1.Postgres{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec: v1alpha1.PostgresSpec{
					Database: name,
				},
				Status: v1alpha1.PostgresStatus{
					Succeeded: true,
					Roles: v1alpha1.PostgresRoles{
						Owner:  name + "-owner",
						Reader: name + "-reader",
						Writer: name + "-writer",
					},
				},
			}

		})

		Context("DropOnDelete is unset", func() {

			BeforeEach(func() {
				initClient(postgresCR, true)
			})

			It("should remove finalizer", func() {
				err := runReconcile(rp, ctx, req)
				// No error should be returned
				Expect(err).NotTo(HaveOccurred())

				// Check updated Postgres
				// somewhat difficult, because it just got deleted and might
				// be gone for real after the last finalizer is removed
				foundPostgres := &v1alpha1.Postgres{}
				err = cl.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, foundPostgres)
				if err != nil {
					Expect(errors.IsNotFound(err)).To(BeTrue())
				} else {
					Expect(foundPostgres.GetFinalizers()).To(BeEmpty())
					Expect(foundPostgres.Status.Succeeded).To(BeTrue())
				}
			})

			It("should not try to delete roles or database", func() {
				// Neither DropRole nor DropDatabase should be called
				pg.EXPECT().DropRole(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				pg.EXPECT().DropDatabase(gomock.Any(), gomock.Any()).Times(0)
				// Call Reconcile
				err := runReconcile(rp, ctx, req)
				Expect(err).NotTo(HaveOccurred())
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
				dropGroupRole = pg.EXPECT().DropRole(name+"-owner", "pguser", name, gomock.Any())
				dropReaderRole = pg.EXPECT().DropRole(name+"-reader", "pguser", name, gomock.Any())
				dropWriterRole = pg.EXPECT().DropRole(name+"-writer", "pguser", name, gomock.Any())
				dropDatabase = pg.EXPECT().DropDatabase(name, gomock.Any())
				// Create Postgres with DropOnDelete == true
				anotherPostgres := postgresCR.DeepCopy()
				anotherPostgres.Spec.DropOnDelete = true
				initClient(anotherPostgres, true)
				foundPostgres := &v1alpha1.Postgres{}
				Expect(cl.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, foundPostgres)).To(BeNil())
			})

			Context("Deletion is successful", func() {

				It("should remove finalizer", func() {
					// No method should return error
					dropGroupRole.Return(nil)
					dropReaderRole.Return(nil)
					dropWriterRole.Return(nil)
					dropDatabase.Return(nil)
					foundPostgres := &v1alpha1.Postgres{}
					Expect(cl.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, foundPostgres)).To(BeNil())
					// Call Reconcile
					err := runReconcile(rp, ctx, req)
					// Patching both the object and its status fails when using the the FakeClient
					//if testEnv != nil {
					Expect(err).NotTo(HaveOccurred())

					// Check updated Postgres
					err = cl.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, foundPostgres)
					if err != nil {
						Expect(errors.IsNotFound(err)).To(BeTrue())
					} else {
						Expect(foundPostgres.GetFinalizers()).To(BeEmpty())
					}
					//}

				})

			})

			Context("Deletion is not successful", func() {

				It("should not remove finalizer when any database action fails", func() {
					// DropDatabase fails
					dropDatabase.Return(fmt.Errorf("Could not drop database"))
					// Call Reconcile
					err := runReconcile(rp, ctx, req)
					// No error should be returned
					Expect(err).To(HaveOccurred())
					// Check updated Postgres
					foundPostgres := &v1alpha1.Postgres{}
					Expect(cl.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, foundPostgres)).To(BeNil())
					Expect(foundPostgres.GetFinalizers()).To(ConsistOf("finalizer.db.movetokube.com"))
				})

			})

			Context("Another Postgres exists with same database", func() {
				var anotherPostgres *v1alpha1.Postgres

				BeforeEach(func() {
					// Create another Postgres with same database name
					Expect(k8sClient.Create(ctx, &corev1.Namespace{
						ObjectMeta: metav1.ObjectMeta{
							Name: "another-namespace",
						},
					})).To(BeNil())
					anotherPostgres = &v1alpha1.Postgres{
						ObjectMeta: metav1.ObjectMeta{
							Name:       "another-database",
							Namespace:  "another-namespace",
							Finalizers: []string{"finalizer.db.movetokube.com"},
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
					initClient(anotherPostgres, true)
				})

				It("should not drop roles or database", func() {
					// Expect no method calls
					dropDatabase.Times(0)
					dropGroupRole.Times(0)
					dropReaderRole.Times(0)
					dropWriterRole.Times(0)
					// Call Reconcile
					err := runReconcile(rp, ctx, req)
					Expect(err).NotTo(HaveOccurred())
				})

			})

		})

	})

	Describe("Checking creation logic", func() {
		var postgresCR *v1alpha1.Postgres

		BeforeEach(func() {
			postgresCR = &v1alpha1.Postgres{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec: v1alpha1.PostgresSpec{
					Database: name,
				},
				Status: v1alpha1.PostgresStatus{},
			}
		})

		Context("MasterRole is unset", func() {

			BeforeEach(func() {
				initClient(postgresCR, false)
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
				err := runReconcile(rp, ctx, req)
				Expect(err).NotTo(HaveOccurred())
			})

		})

		Context("MasterRole is set", func() {

			BeforeEach(func() {
				// Create client
				modPostgres := postgresCR.DeepCopy()
				modPostgres.Spec.MasterRole = "my-master-role"
				initClient(modPostgres, false)
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
				err := runReconcile(rp, ctx, req)
				Expect(err).NotTo(HaveOccurred())
			})

		})

		Context("Correct annotation filter is set", func() {

			BeforeEach(func() {
				// Create client
				modPostgres := postgresCR.DeepCopy()
				modPostgres.Annotations = map[string]string{
					utils.INSTANCE_ANNOTATION: "my-instance",
				}
				rp = &PostgresReconciler{
					Client:         managerClient,
					Scheme:         sc,
					pg:             pg,
					instanceFilter: "my-instance",
				}
				initClient(modPostgres, false)
			})

			It("should create the database", func() {
				pg.EXPECT().CreateGroupRole(gomock.Any()).Return(nil).Times(3)
				pg.EXPECT().CreateDB(name, gomock.Any()).Return(nil)
				// Call Reconcile
				err := runReconcile(rp, ctx, req)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("Incorrect annotation filter is set", func() {

			BeforeEach(func() {
				// Create client
				modPostgres := postgresCR.DeepCopy()
				modPostgres.Annotations = map[string]string{
					utils.INSTANCE_ANNOTATION: "my-instance",
				}
				initClient(modPostgres, false)
			})

			It("should not create the database", func() {
				// Call Reconcile
				err := runReconcile(rp, ctx, req)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("Creation is successful", func() {

			BeforeEach(func() {
				initClient(postgresCR, false)
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
				err := runReconcile(rp, ctx, req)
				// No error should be returned
				Expect(err).NotTo(HaveOccurred())
				// Check updated Postgres
				foundPostgres := &v1alpha1.Postgres{}
				Expect(cl.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, foundPostgres)).To(BeNil())
				Expect(foundPostgres.Status.Roles).To(Equal(expectedRoles))
				Expect(foundPostgres.Status.Succeeded).To(BeTrue())
			})

			It("should set a finalizer", func() {
				expectedFinalizer := "finalizer.db.movetokube.com"
				// Call Reconcile
				err := runReconcile(rp, ctx, req)
				Expect(err).NotTo(HaveOccurred())
				// Check updated Postgres
				foundPostgres := &v1alpha1.Postgres{}
				Expect(cl.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, foundPostgres)).To(BeNil())
				Expect(foundPostgres.GetFinalizers()).To(ContainElement(expectedFinalizer))
			})

		})

		Context("Creation is not successful", func() {

			BeforeEach(func() {
				initClient(postgresCR.DeepCopy(), false)
				// Expected function calls
				pg.EXPECT().CreateGroupRole(gomock.Any()).Return(nil).Times(1)
				pg.EXPECT().CreateDB(name, gomock.Any()).Return(fmt.Errorf("Could not create database"))
			})

			It("should not mark status as successful", func() {
				expectedRoles := v1alpha1.PostgresRoles{
					Owner:  name + "-group",
					Reader: "",
					Writer: "",
				}
				// Call Reconcile
				_, err := rp.Reconcile(ctx, req)
				Expect(err).To(HaveOccurred())
				// Check updated Postgres
				foundPostgres := &v1alpha1.Postgres{}
				Expect(cl.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, foundPostgres)).To(BeNil())
				Expect(foundPostgres.Status.Roles).To(Equal(expectedRoles))
				Expect(foundPostgres.Status.Succeeded).To(BeFalse())
			})

		})

	})

	Describe("Checking extensions logic", func() {
		var postgresCR *v1alpha1.Postgres
		BeforeEach(func() {
			postgresCR = &v1alpha1.Postgres{
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
		})

		Context("Postgres has no extensions", func() {

			BeforeEach(func() {
				initClient(postgresCR, false)
			})

			It("should not try to create extensions", func() {
				// CreateExtension should not be called
				pg.EXPECT().CreateExtension(name, gomock.Any(), gomock.Any()).Times(0)
				// Call Reconcile
				err := runReconcile(rp, ctx, req)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not set status", func() {
				// Call reconcile
				err := runReconcile(rp, ctx, req)
				Expect(err).NotTo(HaveOccurred())
				// Check updated Postgres
				foundPostgres := &v1alpha1.Postgres{}
				Expect(cl.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, foundPostgres)).To(BeNil())
				Expect(foundPostgres.Status.Extensions).To(BeEmpty())
			})

		})

		Context("Postgres has extensions", func() {

			BeforeEach(func() {
				// Add extensions to Postgres object
				extPostgres := postgresCR.DeepCopy()
				extPostgres.Spec.Extensions = []string{"pg_stat_statements", "hstore"}
				initClient(extPostgres, false)
			})

			Context("Creation is successful", func() {

				BeforeEach(func() {
					// Expected method calls
					pg.EXPECT().CreateExtension(name, "pg_stat_statements", gomock.Any()).Return(nil).Times(1)
					pg.EXPECT().CreateExtension(name, "hstore", gomock.Any()).Return(nil).Times(1)
				})

				It("should update status", func() {
					// Call reconcile
					err := runReconcile(rp, ctx, req)
					Expect(err).NotTo(HaveOccurred())
					// Check updated Postgres
					foundPostgres := &v1alpha1.Postgres{}
					Expect(cl.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, foundPostgres)).To(BeNil())
					Expect(foundPostgres.Status.Extensions).To(ConsistOf("pg_stat_statements", "hstore"))
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
					err := runReconcile(rp, ctx, req)
					Expect(err).NotTo(HaveOccurred())
					// Check updated Postgres
					foundPostgres := &v1alpha1.Postgres{}
					Expect(cl.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, foundPostgres)).To(BeNil())
					Expect(foundPostgres.Status.Extensions).To(ConsistOf("hstore"))
				})

			})

		})

		Context("Subset of extensions already created", func() {

			BeforeEach(func() {
				// Add extensions to Postgres object
				extPostgres := postgresCR.DeepCopy()
				extPostgres.Spec.Extensions = []string{"pg_stat_statements", "hstore"}
				extPostgres.Status.Extensions = []string{"hstore"}
				initClient(extPostgres, false)
			})

			Context("Creation is successful", func() {

				It("should not recreate existing extension", func() {
					// Expected method calls
					pg.EXPECT().CreateExtension(name, "pg_stat_statements", gomock.Any()).Return(nil).Times(1)
					pg.EXPECT().CreateExtension(name, "hstore", gomock.Any()).Times(0)
					// Call reconcile
					err := runReconcile(rp, ctx, req)
					Expect(err).NotTo(HaveOccurred())
					// Check updated Postgres
					foundPostgres := &v1alpha1.Postgres{}
					Expect(cl.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, foundPostgres)).To(BeNil())
					Expect(foundPostgres.Status.Extensions).To(ConsistOf("hstore", "pg_stat_statements"))
				})

			})

		})

	})

	Describe("Checking schemas logic", func() {
		var postgresCR *v1alpha1.Postgres
		BeforeEach(func() {
			postgresCR = &v1alpha1.Postgres{
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
		})

		Context("Postgres has no schemas", func() {

			BeforeEach(func() {
				initClient(postgresCR, false)
			})

			It("should not try to create schemas", func() {
				// CreateSchema should not be called
				pg.EXPECT().CreateSchema(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				// Call Reconcile
				err := runReconcile(rp, ctx, req)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not set status", func() {
				// Call reconcile
				err := runReconcile(rp, ctx, req)
				Expect(err).NotTo(HaveOccurred())
				// Check updated Postgres
				foundPostgres := &v1alpha1.Postgres{}
				Expect(cl.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, foundPostgres)).To(BeNil())
				Expect(foundPostgres.Status.Schemas).To(BeEmpty())
			})

		})

		Context("Postgres has schemas", func() {

			BeforeEach(func() {
				// Add schemas to Postgres object
				schemaPostgres := postgresCR.DeepCopy()
				schemaPostgres.Spec.Schemas = []string{"customers", "stores"}
				initClient(schemaPostgres, false)
			})

			Context("Creation is successful", func() {

				BeforeEach(func() {
					// Expected method calls
					// customers schema
					pg.EXPECT().CreateSchema(name, name+"-group", "customers", gomock.Any()).Return(nil).Times(1)
					pg.EXPECT().SetSchemaPrivileges(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
					// stores schema
					pg.EXPECT().CreateSchema(name, name+"-group", "stores", gomock.Any()).Return(nil).Times(1)
					pg.EXPECT().SetSchemaPrivileges(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				})

				It("should update status", func() {
					// Call reconcile
					err := runReconcile(rp, ctx, req)
					Expect(err).NotTo(HaveOccurred())
					// Check updated Postgres
					foundPostgres := &v1alpha1.Postgres{}
					Expect(cl.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, foundPostgres)).To(BeNil())
					Expect(foundPostgres.Status.Schemas).To(ConsistOf("customers", "stores"))
				})

			})

			Context("Creation is not successful", func() {

				BeforeEach(func() {
					// Expected method calls
					// customers schema errors
					pg.EXPECT().CreateSchema(name, name+"-group", "customers", gomock.Any()).Return(fmt.Errorf("Could not create schema")).Times(1)
					pg.EXPECT().SetSchemaPrivileges(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
					// stores schema
					pg.EXPECT().CreateSchema(name, name+"-group", "stores", gomock.Any()).Return(nil).Times(1)
					pg.EXPECT().SetSchemaPrivileges(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				})

				It("should update status", func() {
					// Call reconcile
					err := runReconcile(rp, ctx, req)
					Expect(err).NotTo(HaveOccurred())
					// Check updated Postgres
					foundPostgres := &v1alpha1.Postgres{}
					Expect(cl.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, foundPostgres)).To(BeNil())
					Expect(foundPostgres.Status.Schemas).To(ConsistOf("stores"))
				})

			})

		})

		Context("Subset of schema already created", func() {

			BeforeEach(func() {
				// Add schemas to Postgres object
				schemaPostgres := postgresCR.DeepCopy()
				schemaPostgres.Spec.Schemas = []string{"customers", "stores"}
				schemaPostgres.Status.Schemas = []string{"stores"}
				initClient(schemaPostgres, false)
			})

			Context("Creation is successful", func() {

				It("should not recreate existing schema", func() {
					// customers schema
					pg.EXPECT().CreateSchema(name, name+"-group", "customers", gomock.Any()).Return(nil).Times(1)
					pg.EXPECT().SetSchemaPrivileges(gomock.Any(), gomock.Any()).Return(nil).Times(3)
					// stores schema already exists
					pg.EXPECT().CreateSchema(name, name+"-group", "stores", gomock.Any()).Times(0)
					pg.EXPECT().SetSchemaPrivileges(gomock.Any(), gomock.Any()).Return(nil).Times(0)
					// Call reconcile
					err := runReconcile(rp, ctx, req)
					Expect(err).NotTo(HaveOccurred())
					// Check updated Postgres
					foundPostgres := &v1alpha1.Postgres{}
					Expect(cl.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, foundPostgres)).To(BeNil())
					Expect(foundPostgres.Status.Schemas).To(ConsistOf("stores", "customers"))
				})

			})

		})

	})

})
