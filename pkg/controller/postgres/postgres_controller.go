package postgres

import (
	"context"
	goerr "errors"
	"fmt"
	"github.com/go-logr/logr"
	dbv1alpha1 "github.com/movetokube/postgres-operator/pkg/apis/db/v1alpha1"
	"github.com/movetokube/postgres-operator/pkg/postgres"
	"github.com/movetokube/postgres-operator/pkg/utils"
	"k8s.io/apimachinery/pkg/api/errors"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_postgres")

// Add creates a new Postgres Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	pgHost := utils.MustGetEnv("POSTGRES_HOST")
	pgUser := utils.MustGetEnv("POSTGRES_USER")
	pgPass := utils.MustGetEnv("POSTGRES_PASS")
	pgUriArgs := utils.MustGetEnv("POSTGRES_URI_ARGS")
	pg, err := postgres.NewPG(pgHost, pgUser, pgPass, pgUriArgs, log.WithName("postgres"))
	if err != nil {
		return nil
	}

	return &ReconcilePostgres{
		client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
		pg:     pg,
		pgHost: pgHost,
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	if r == nil {
		return errors.NewInternalError(goerr.New("failed to get reconciler"))
	}
	// Create a new controller
	c, err := controller.New("postgres-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Postgres
	err = c.Watch(&source.Kind{Type: &dbv1alpha1.Postgres{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcilePostgres{}

// ReconcilePostgres reconciles a Postgres object
type ReconcilePostgres struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
	pg     postgres.PG
	pgHost string
}

// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcilePostgres) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Postgres")

	// Fetch the Postgres instance
	instance := &dbv1alpha1.Postgres{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// deletion logic
	if instance.GetDeletionTimestamp() != nil {
		if r.shouldDropDB(instance, reqLogger) && instance.Status.Succeeded && instance.Status.PostgresRole != "" {
			err := r.pg.DropRole(instance.Status.PostgresRole, r.pg.GetUser(), instance.Spec.Database, reqLogger)
			if err != nil {
				return reconcile.Result{}, err
			}
			err = r.pg.DropDatabase(instance.Spec.Database, reqLogger)
			if err != nil {
				return reconcile.Result{}, err
			}
		}
		instance.SetFinalizers(nil)

		// Update CR
		err = r.client.Update(context.TODO(), instance)
		if err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}

	// creation logic
	if instance.Status.PostgresRole == "" {
		if instance.Spec.MasterRole == "" {
			instance.Spec.MasterRole = fmt.Sprintf("%s-group", instance.Spec.Database)
		}
		err = r.pg.CreateGroupRole(instance.Spec.MasterRole)
		if err != nil {
			return r.requeue(instance, errors.NewInternalError(err))
		}
		err = r.pg.CreateDB(instance.Spec.Database, instance.Spec.MasterRole)
		if err != nil {
			return r.requeue(instance, errors.NewInternalError(err))
		}

		instance.Status.PostgresRole = instance.Spec.MasterRole
		instance.Status.Succeeded = true
		err = r.client.Update(context.TODO(), instance)
		if err != nil {
			return r.requeue(instance, err)
		}
	}

	err = r.addFinalizer(reqLogger, instance)
	if err != nil {
		return r.requeue(instance, err)
	}

	reqLogger.Info("reconciler done", "CR.Namespace", instance.Namespace, "CR.Name", instance.Name)
	return reconcile.Result{}, nil
}

func (r *ReconcilePostgres) addFinalizer(reqLogger logr.Logger, m *dbv1alpha1.Postgres) error {
	if len(m.GetFinalizers()) < 1 && m.GetDeletionTimestamp() == nil {
		reqLogger.Info("adding Finalizer for Postgres")
		m.SetFinalizers([]string{"finalizer.db.movetokube.com"})

		// Update CR
		err := r.client.Update(context.TODO(), m)
		if err != nil {
			reqLogger.Error(err, "failed to update Posgres with finalizer")
			return err
		}
	}
	return nil
}

func (r *ReconcilePostgres) requeue(cr *dbv1alpha1.Postgres, reason error) (reconcile.Result, error) {
	cr.Status.Succeeded = false
	err := r.client.Update(context.TODO(), cr)
	if err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, reason
}

func (r *ReconcilePostgres) finish(cr *dbv1alpha1.Postgres) (reconcile.Result, error) {
	cr.Status.Succeeded = true
	err := r.client.Update(context.TODO(), cr)
	if err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

func (r *ReconcilePostgres) shouldDropDB(cr *dbv1alpha1.Postgres, logger logr.Logger) bool {
	// If DropOnDelete is false we don't need to check any further
	if !cr.Spec.DropOnDelete {
		return false
	}
	// Get a list of all Postgres
	dbs := dbv1alpha1.PostgresList{}
	err := r.client.List(context.TODO(), &client.ListOptions{}, &dbs)
	if err != nil {
		logger.Info(fmt.Sprintf("%v", err))
		return true
	}

	for _, db := range dbs.Items {
		// Skip database if it's the same as the one we're deleting
		if db.Name == cr.Name && db.Namespace == cr.Namespace {
			continue
		}
		// There already exists another Postgres who has the same database
		// Let's not drop the database
		if db.Spec.Database == cr.Spec.Database {
			return false
		}
	}

	return true
}
