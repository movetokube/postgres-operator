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

package controllers

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/movetokube/postgres-operator/postgres"
	"github.com/movetokube/postgres-operator/utils"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	dbmovetokubecomv1alpha1 "github.com/movetokube/postgres-operator/api/v1alpha1"

	kerrors "k8s.io/apimachinery/pkg/util/errors"
)

// PostgresReconciler reconciles a Postgres object
type PostgresReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	pg             postgres.PG
	pgHost         string
	instanceFilter string
}

//+kubebuilder:rbac:groups=db.movetokube.com,resources=postgres,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=db.movetokube.com,resources=postgres/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=db.movetokube.com,resources=postgres/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.

// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.2/pkg/reconcile
func (r *PostgresReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	reqLogger := log.FromContext(ctx, "Request.Namespace", req.Namespace, "Request.Name", req.Name)
	reqLogger.Info("Reconciling Postgres")

	instance := &dbmovetokubecomv1alpha1.Postgres{}
	// Fetch the Postgres instance
	err := r.Client.Get(context.TODO(), req.NamespacedName, instance)
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
	before := instance.DeepCopyObject()
	// Patch after every reconcile loop, if needed
	defer func() {
		err = utils.Patch(r.Client, ctx, before, instance)
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
		instance.SetFinalizers(nil)

		return ctrl.Result{}, nil
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

	err = r.addFinalizer(reqLogger, instance)
	if err != nil {
		return r.requeue(instance, err)
	}

	reqLogger.Info("reconciler done", "CR.Namespace", instance.Namespace, "CR.Name", instance.Name)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PostgresReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dbmovetokubecomv1alpha1.Postgres{}).
		Complete(r)
}

func (r *PostgresReconciler) addFinalizer(reqLogger logr.Logger, m *dbmovetokubecomv1alpha1.Postgres) error {
	if len(m.GetFinalizers()) < 1 && m.GetDeletionTimestamp() == nil {
		reqLogger.Info("adding Finalizer for Postgres")
		m.SetFinalizers([]string{"finalizer.db.movetokube.com"})
	}
	return nil
}

func (r *PostgresReconciler) requeue(cr *dbmovetokubecomv1alpha1.Postgres, reason error) (reconcile.Result, error) {
	cr.Status.Succeeded = false
	return reconcile.Result{}, reason
}

func (r *PostgresReconciler) shouldDropDB(cr *dbmovetokubecomv1alpha1.Postgres, logger logr.Logger) bool {
	// If DropOnDelete is false we don't need to check any further
	if !cr.Spec.DropOnDelete {
		return false
	}
	// Get a list of all Postgres
	dbs := dbmovetokubecomv1alpha1.PostgresList{}
	err := r.Client.List(context.TODO(), &dbs, &client.ListOptions{})
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
