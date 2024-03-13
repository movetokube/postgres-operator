package postgres

import (
	"context"
	goerr "errors"
	"fmt"

	"github.com/movetokube/postgres-operator/pkg/config"

	"github.com/go-logr/logr"
	dbv1alpha1 "github.com/movetokube/postgres-operator/pkg/apis/db/v1alpha1"
	"github.com/movetokube/postgres-operator/pkg/postgres"
	"github.com/movetokube/postgres-operator/pkg/utils"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
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
	c := config.Get()
	pg, err := postgres.NewPG(c.PostgresHost, c.PostgresUser, c.PostgresPass, c.PostgresUriArgs, c.PostgresDefaultDb, c.CloudProvider, log.WithName("postgres"))
	if err != nil {
		return nil
	}

	return &ReconcilePostgres{
		client:         mgr.GetClient(),
		scheme:         mgr.GetScheme(),
		pg:             pg,
		pgHost:         c.PostgresHost,
		instanceFilter: c.AnnotationFilter,
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
	client         client.Client
	scheme         *runtime.Scheme
	pg             postgres.PG
	pgHost         string
	instanceFilter string
}

// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcilePostgres) Reconcile(request reconcile.Request) (_ reconcile.Result, reterr error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Postgres")

	instance := &dbv1alpha1.Postgres{}
	// Fetch the Postgres instance
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

	if !utils.MatchesInstanceAnnotation(instance.Annotations, r.instanceFilter) {
		return reconcile.Result{}, nil
	}
	before := instance.DeepCopyObject()
	// Patch after every reconcile loop, if needed
	defer func() {
		err = utils.Patch(r.client, context.TODO(), before, instance)
		if err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	// deletion logic
	if !instance.GetDeletionTimestamp().IsZero() {
		if r.shouldDropDB(instance, reqLogger) && instance.Status.Succeeded {
			if instance.Status.Roles.Owner != "" {
				err := r.pg.DropRole(instance.Status.Roles.Owner, r.pg.GetUser(), instance.Spec.Database, reqLogger)
				if err != nil {
					return reconcile.Result{}, err
				}
				instance.Status.Roles.Owner = ""
			}
			if instance.Status.Roles.Reader != "" {
				err = r.pg.DropRole(instance.Status.Roles.Reader, r.pg.GetUser(), instance.Spec.Database, reqLogger)
				if err != nil {
					return reconcile.Result{}, err
				}
				instance.Status.Roles.Reader = ""
			}
			if instance.Status.Roles.Writer != "" {
				err = r.pg.DropRole(instance.Status.Roles.Writer, r.pg.GetUser(), instance.Spec.Database, reqLogger)
				if err != nil {
					return reconcile.Result{}, err
				}
				instance.Status.Roles.Writer = ""
			}
			err = r.pg.DropDatabase(instance.Spec.Database, reqLogger)
			if err != nil {
				return reconcile.Result{}, err
			}
		}
		instance.SetFinalizers(nil)

		return reconcile.Result{}, nil
	}

	// creation logic
	if !instance.Status.Succeeded {
		owner := instance.Spec.MasterRole
		if owner == "" {
			owner = fmt.Sprintf("%s-group", instance.Spec.Database)
		}
		// Create owner role
		err = r.pg.CreateGroupRole(owner)
		if err != nil {
			return r.requeue(instance, errors.NewInternalError(err))
		}
		// Create database
		err = r.pg.CreateDB(instance.Spec.Database, owner)
		if err != nil {
			return r.requeue(instance, errors.NewInternalError(err))
		}

		// Create reader role
		reader := fmt.Sprintf("%s-reader", instance.Spec.Database)
		err = r.pg.CreateGroupRole(reader)
		if err != nil {
			return r.requeue(instance, errors.NewInternalError(err))
		}

		// Create writer role
		writer := fmt.Sprintf("%s-writer", instance.Spec.Database)
		err = r.pg.CreateGroupRole(writer)
		if err != nil {
			return r.requeue(instance, errors.NewInternalError(err))
		}

		instance.Status.Roles.Owner = owner
		instance.Status.Roles.Reader = reader
		instance.Status.Roles.Writer = writer
		instance.Status.Succeeded = true
	}
	// create extensions
	for _, extension := range instance.Spec.Extensions {
		// Check if extension is already added. Skip if already is added.
		if utils.ListContains(instance.Status.Extensions, extension) {
			continue
		}
		// Execute create extension SQL statement
		err = r.pg.CreateExtension(instance.Spec.Database, extension, reqLogger)
		if err != nil {
			reqLogger.Error(err, fmt.Sprintf("Could not add extensions %s", extension))
			continue
		}
		instance.Status.Extensions = append(instance.Status.Extensions, extension)
	}
	// create schemas
	var (
		database    = instance.Spec.Database
		owner       = instance.Status.Roles.Owner
		reader      = instance.Status.Roles.Reader
		writer      = instance.Status.Roles.Writer
		readerPrivs = "SELECT"
		writerPrivs = "SELECT,INSERT,DELETE,UPDATE"
	)
	for _, schema := range instance.Spec.Schemas {
		// Schema was previously created
		if utils.ListContains(instance.Status.Schemas, schema) {
			continue
		}

		// Create schema
		err = r.pg.CreateSchema(database, owner, schema, reqLogger)
		if err != nil {
			reqLogger.Error(err, fmt.Sprintf("Could not create schema %s", schema))
			continue
		}

		// Set privileges on schema
		schemaPrivilegesReader := postgres.PostgresSchemaPrivileges{database, owner, reader, schema, readerPrivs, false}
		err = r.pg.SetSchemaPrivileges(schemaPrivilegesReader, reqLogger)
		if err != nil {
			reqLogger.Error(err, fmt.Sprintf("Could not give %s permissions \"%s\"", reader, readerPrivs))
			continue
		}
		schemaPrivilegesWriter := postgres.PostgresSchemaPrivileges{database, owner, writer, schema, readerPrivs, true}
		err = r.pg.SetSchemaPrivileges(schemaPrivilegesWriter, reqLogger)
		if err != nil {
			reqLogger.Error(err, fmt.Sprintf("Could not give %s permissions \"%s\"", writer, writerPrivs))
			continue
		}
		schemaPrivilegesOwner := postgres.PostgresSchemaPrivileges{database, owner, owner, schema, readerPrivs, true}
		err = r.pg.SetSchemaPrivileges(schemaPrivilegesOwner, reqLogger)
		if err != nil {
			reqLogger.Error(err, fmt.Sprintf("Could not give %s permissions \"%s\"", writer, writerPrivs))
			continue
		}

		instance.Status.Schemas = append(instance.Status.Schemas, schema)
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
		m.SetFinalizers([]string{"foregroundDeletion"})
	}
	return nil
}

func (r *ReconcilePostgres) requeue(cr *dbv1alpha1.Postgres, reason error) (reconcile.Result, error) {
	cr.Status.Succeeded = false
	return reconcile.Result{}, reason
}

func (r *ReconcilePostgres) shouldDropDB(cr *dbv1alpha1.Postgres, logger logr.Logger) bool {
	// If DropOnDelete is false we don't need to check any further
	if !cr.Spec.DropOnDelete {
		return false
	}
	// Get a list of all Postgres
	dbs := dbv1alpha1.PostgresList{}
	err := r.client.List(context.TODO(), &dbs, &client.ListOptions{})
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
