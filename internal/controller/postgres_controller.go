package controller

import (
	"context"
	"fmt"
	"slices"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/go-logr/logr"
	dbv1alpha1 "github.com/movetokube/postgres-operator/api/v1alpha1"
	"github.com/movetokube/postgres-operator/pkg/config"
	"github.com/movetokube/postgres-operator/pkg/postgres"
	"github.com/movetokube/postgres-operator/pkg/utils"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
)

// PostgresReconciler reconciles a Postgres object
type PostgresReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	pg     postgres.PG
	// pgHost         string
	instanceFilter string
}

// NewPostgresReconciler returns a new reconcile.Reconciler
func NewPostgresReconciler(mgr manager.Manager, c *config.Cfg, pg postgres.PG) *PostgresReconciler {
	return &PostgresReconciler{
		Client:         mgr.GetClient(),
		Scheme:         mgr.GetScheme(),
		pg:             pg,
		instanceFilter: c.AnnotationFilter,
	}
}

// +kubebuilder:rbac:groups=db.movetokube.com,resources=postgres,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=db.movetokube.com,resources=postgres/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=db.movetokube.com,resources=postgres/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Postgres object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/reconcile
func (r *PostgresReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	reqLogger := log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	reqLogger.Info("Reconciling Postgres")

	instance := &dbv1alpha1.Postgres{}
	// Fetch the Postgres instance
	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	if !utils.MatchesInstanceAnnotation(instance.Annotations, r.instanceFilter) {
		return ctrl.Result{}, nil
	}
	before := instance.DeepCopy()

	// deletion logic
	if !instance.GetDeletionTimestamp().IsZero() {
		if r.shouldDropDB(ctx, instance, reqLogger) && instance.Status.Succeeded {
			if instance.Status.Roles.Owner != "" {
				err := r.pg.DropRole(instance.Status.Roles.Owner, r.pg.GetUser(), instance.Spec.Database, reqLogger)
				if err != nil {
					return ctrl.Result{}, err
				}
				instance.Status.Roles.Owner = ""
			}
			if instance.Status.Roles.Reader != "" {
				err = r.pg.DropRole(instance.Status.Roles.Reader, r.pg.GetUser(), instance.Spec.Database, reqLogger)
				if err != nil {
					return ctrl.Result{}, err
				}
				instance.Status.Roles.Reader = ""
			}
			if instance.Status.Roles.Writer != "" {
				err = r.pg.DropRole(instance.Status.Roles.Writer, r.pg.GetUser(), instance.Spec.Database, reqLogger)
				if err != nil {
					return ctrl.Result{}, err
				}
				instance.Status.Roles.Writer = ""
			}
			err = r.pg.DropDatabase(instance.Spec.Database, reqLogger)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
		err = r.Status().Patch(ctx, instance, client.MergeFrom(before))
		if err != nil {
			reqLogger.Error(err, "could not update db status")
		}
		controllerutil.RemoveFinalizer(instance, "finalizer.db.movetokube.com")
		err = r.Patch(ctx, instance, client.MergeFrom(before))
		if err != nil {
			reqLogger.Error(err, "could not remove finalizer")
		}

		return ctrl.Result{}, nil
	}

	// Patch after every reconcile loop, if needed
	requeue := func(err error) (ctrl.Result, error) {
		reqLogger.Error(err, "Requeuing...")
		instance.Status.Succeeded = false
		updateErr := r.Status().Patch(ctx, instance, client.MergeFrom(before))
		if updateErr != nil {
			err = kerrors.NewAggregate([]error{err, updateErr})
		}
		return ctrl.Result{Requeue: true}, err
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
			return requeue(errors.NewInternalError(err))
		}
		instance.Status.Roles.Owner = owner

		// Create database
		err = r.pg.CreateDB(instance.Spec.Database, owner)
		if err != nil {
			reqLogger.Error(err, "Could not create DB")
			return requeue(errors.NewInternalError(err))
		}

		// Create reader role
		reader := fmt.Sprintf("%s-reader", instance.Spec.Database)
		err = r.pg.CreateGroupRole(reader)
		if err != nil {
			return requeue(errors.NewInternalError(err))
		}
		instance.Status.Roles.Reader = reader

		// Create writer role
		writer := fmt.Sprintf("%s-writer", instance.Spec.Database)
		err = r.pg.CreateGroupRole(writer)
		if err != nil {
			return requeue(errors.NewInternalError(err))
		}
		instance.Status.Roles.Writer = writer
		instance.Status.Succeeded = true
	}
	// create extensions
	for _, extension := range instance.Spec.Extensions {
		// Check if extension is already added. Skip if already is added.
		if slices.Contains(instance.Status.Extensions, extension) {
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
		if slices.Contains(instance.Status.Schemas, schema) {
			continue
		}

		// Create schema
		err = r.pg.CreateSchema(database, owner, schema, reqLogger)
		if err != nil {
			reqLogger.Error(err, fmt.Sprintf("Could not create schema %s", schema))
			continue
		}

		// Set privileges on schema
		schemaPrivilegesReader := postgres.PostgresSchemaPrivileges{
			DB:           database,
			Role:         reader,
			Schema:       schema,
			Privs:        readerPrivs,
			CreateSchema: false,
		}
		err = r.pg.SetSchemaPrivileges(schemaPrivilegesReader, reqLogger)
		if err != nil {
			reqLogger.Error(err, fmt.Sprintf("Could not give %s permissions \"%s\"", reader, readerPrivs))
			continue
		}
		schemaPrivilegesWriter := postgres.PostgresSchemaPrivileges{
			DB:           database,
			Role:         writer,
			Schema:       schema,
			Privs:        writerPrivs,
			CreateSchema: true,
		}
		err = r.pg.SetSchemaPrivileges(schemaPrivilegesWriter, reqLogger)
		if err != nil {
			reqLogger.Error(err, fmt.Sprintf("Could not give %s permissions \"%s\"", writer, writerPrivs))
			continue
		}
		schemaPrivilegesOwner := postgres.PostgresSchemaPrivileges{
			DB:           database,
			Role:         owner,
			Schema:       schema,
			Privs:        writerPrivs,
			CreateSchema: true,
		}
		err = r.pg.SetSchemaPrivileges(schemaPrivilegesOwner, reqLogger)
		if err != nil {
			reqLogger.Error(err, fmt.Sprintf("Could not give %s permissions \"%s\"", writer, writerPrivs))
			continue
		}

		instance.Status.Schemas = append(instance.Status.Schemas, schema)
	}
	err = r.Status().Patch(ctx, instance, client.MergeFrom(before))
	if err != nil {
		return requeue(err)
	}
	before = instance.DeepCopy()
	if controllerutil.AddFinalizer(instance, "finalizer.db.movetokube.com") {
		err = r.Patch(ctx, instance, client.MergeFrom(before))
		if err != nil {
			return requeue(err)
		}
	}

	reqLogger.Info("reconciler done", "CR.Namespace", instance.Namespace, "CR.Name", instance.Name)
	return ctrl.Result{}, nil
}
func (r *PostgresReconciler) addFinalizer(reqLogger logr.Logger, m *dbv1alpha1.Postgres) error {
	if len(m.GetFinalizers()) < 1 && m.GetDeletionTimestamp() == nil {
		reqLogger.Info("adding Finalizer for Postgres")
		m.SetFinalizers([]string{"finalizer.db.movetokube.com"})
	}
	return nil
}
func (r *PostgresReconciler) requeue(cr *dbv1alpha1.Postgres, reason error) (ctrl.Result, error) {
	cr.Status.Succeeded = false
	return ctrl.Result{}, reason
}

func (r *PostgresReconciler) shouldDropDB(ctx context.Context, cr *dbv1alpha1.Postgres, logger logr.Logger) bool {
	// If DropOnDelete is false we don't need to check any further
	if !cr.Spec.DropOnDelete {
		return false
	}
	// Get a list of all Postgres
	dbs := dbv1alpha1.PostgresList{}
	err := r.List(ctx, &dbs, &client.ListOptions{})
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

// SetupWithManager sets up the controller with the Manager.
func (r *PostgresReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dbv1alpha1.Postgres{}).
		Complete(r)
}
