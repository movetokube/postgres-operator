/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/movetokube/postgres-operator/internal/postgres"
	"github.com/movetokube/postgres-operator/internal/utils"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	dbmovetokubecomv1alpha1 "github.com/movetokube/postgres-operator/api/v1alpha1"

	kerrors "k8s.io/apimachinery/pkg/util/errors"
)

// PostgresReconciler reconciles a Postgres object
type PostgresReconciler struct {
	client.Client
	scheme         *runtime.Scheme
	pg             postgres.PG
	pgHost         string
	prefix         string
	instanceFilter string
	keepSecretName bool
}

// NewPostgresReconciler returns a new reconcile.Reconciler
func NewPostgresReconciler(mgr manager.Manager, c *utils.Cfg, pg postgres.PG) *PostgresReconciler {
	return &PostgresReconciler{
		Client:         mgr.GetClient(),
		scheme:         mgr.GetScheme(),
		pg:             pg,
		instanceFilter: c.AnnotationFilter,
		keepSecretName: c.KeepSecretName,
	}
}

func (r *PostgresReconciler) GetPrefixedDbName(dbname string) string {
	return r.prefix + dbname
}

//+kubebuilder:rbac:groups=db.movetokube.com,resources=postgres,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=db.movetokube.com,resources=postgres/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=db.movetokube.com,resources=postgres/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.

// Reconcile the PostgresUser, see
// https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.15.0/pkg/reconcile
func (r *PostgresReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	reqLogger := log.FromContext(ctx)
	reqLogger.Info("Reconciling Postgres", "Request.Namespace", req.Namespace, "Request.Name", req.Name)

	instance := &dbmovetokubecomv1alpha1.Postgres{}
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
		reqLogger.Info("Doesn't match instance annotation")
		return ctrl.Result{}, nil
	}

	before := instance.DeepCopy()

	prefixedDbName := r.GetPrefixedDbName(instance.Spec.Database)

	// deletion logic
	// indicated by the deletion timestamp being set.
	if !instance.GetDeletionTimestamp().IsZero() {
		reqLogger.Info("resource was deleted")
		if r.shouldDropDB(instance, reqLogger) && instance.Status.Succeeded {
			reqLogger.Info("Dropping DB", "dbname", prefixedDbName)
			for _, field := range []*string{
				&instance.Status.Roles.Owner,
				&instance.Status.Roles.Reader,
				&instance.Status.Roles.Writer,
			} {
				if *field != "" {
					reqLogger.Info("Dropping Role", "role", *field)
					err := r.pg.DropRole(*field, r.pg.GetUser(), prefixedDbName, reqLogger)
					if err != nil {
						return ctrl.Result{}, err
					}
					*field = ""
				}
			}
			err = r.pg.DropDatabase(prefixedDbName, reqLogger)
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
		return ctrl.Result{}, err
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
		reqLogger.Info("creating database and roles")
		owner := instance.Spec.MasterRole
		if owner == "" {
			owner = prefixedDbName + "-group"
		}
		// Create owner role
		err = r.pg.CreateGroupRole(owner)
		if err != nil {
			return requeue(errors.NewInternalError(err))
		}
		instance.Status.Roles.Owner = owner

		// Create database
		err = r.pg.CreateDB(prefixedDbName, owner)
		if err != nil {
			reqLogger.Error(err, "Could not create DB")
			return requeue(errors.NewInternalError(err))
		}
		instance.Status.DbName = prefixedDbName

		// Create reader role
		reader := prefixedDbName + "-reader"
		err = r.pg.CreateGroupRole(reader)
		if err != nil {
			return requeue(errors.NewInternalError(err))
		}
		instance.Status.Roles.Reader = reader

		// Create writer role
		writer := prefixedDbName + "-writer"
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
		if utils.ListContains(instance.Status.Extensions, extension) {
			continue
		}
		reqLogger.Info("creating extension", "extension", extension)
		// Execute create extension SQL statement
		err = r.pg.CreateExtension(prefixedDbName, extension, reqLogger)
		if err != nil {
			reqLogger.Error(err, fmt.Sprintf("Could not add extensions %s", extension))
			continue
		}
		instance.Status.Extensions = append(instance.Status.Extensions, extension)
	}
	// create schemas
	var (
		database    = prefixedDbName
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

		reqLogger.Info("creating schema", "schema", schema)
		// Create schema
		err = r.pg.CreateSchema(database, owner, schema, reqLogger)
		if err != nil {
			reqLogger.Error(err, fmt.Sprintf("Could not create schema %s", schema))
			continue
		}

		// Set privileges on schema
		err = r.pg.SetSchemaPrivileges(database, owner, reader, schema, readerPrivs, reqLogger)
		if err != nil {
			reqLogger.Error(err, fmt.Sprintf("Could not give %s permissions \"%s\"", reader, readerPrivs))
			continue
		}
		err = r.pg.SetSchemaPrivileges(database, owner, writer, schema, writerPrivs, reqLogger)
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

	reqLogger.Info("reconciler done")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PostgresReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dbmovetokubecomv1alpha1.Postgres{}).
		Complete(r)
}

func (r *PostgresReconciler) shouldDropDB(cr *dbmovetokubecomv1alpha1.Postgres, logger logr.Logger) bool {
	// If DropOnDelete is false we don't need to check any further
	if !cr.Spec.DropOnDelete {
		return false
	}
	// Get a list of all Postgres
	dbs := dbmovetokubecomv1alpha1.PostgresList{}
	err := r.List(context.TODO(), &dbs, &client.ListOptions{})
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
		if db.Status.DbName == cr.Status.DbName {
			logger.Info("DB still used", "othercr", db)
			return false
		}
	}

	return true
}
