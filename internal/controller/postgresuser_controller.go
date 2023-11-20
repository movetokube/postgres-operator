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

	dbv1alpha1 "github.com/movetokube/postgres-operator/api/v1alpha1"
	"github.com/movetokube/postgres-operator/internal/postgres"
	"github.com/movetokube/postgres-operator/internal/utils"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// blank assignment to verify that PostgresUserReconciler implements reconcile.Reconciler
var _ reconcile.Reconciler = &PostgresUserReconciler{}

// PostgresUserReconciler reconciles a PostgresUser object
type PostgresUserReconciler struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client.Client
	scheme         *runtime.Scheme
	pg             postgres.PG
	instanceFilter string
	keepSecretName bool // use secret name as defined in PostgresUserSpec
}

// NewPostgresUserReconciler returns a new reconcile.Reconciler
func NewPostgresUserReconciler(mgr manager.Manager, c *utils.Cfg, pg postgres.PG) *PostgresUserReconciler {
	return &PostgresUserReconciler{
		Client:         mgr.GetClient(),
		scheme:         mgr.GetScheme(),
		pg:             pg,
		instanceFilter: c.AnnotationFilter,
		keepSecretName: c.KeepSecretName,
	}
}

//+kubebuilder:rbac:groups=db.movetokube.com,resources=postgresusers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=db.movetokube.com,resources=postgresusers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=db.movetokube.com,resources=postgresusers/finalizers,verbs=update

// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *PostgresUserReconciler) Reconcile(ctx context.Context, req reconcile.Request) (ctrl.Result, error) {
	reqLogger := log.FromContext(ctx, "Request.Namespace", req.Namespace, "Request.Name", req.Name)

	// Fetch the PostgresUser instance
	instance := &dbv1alpha1.PostgresUser{}
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

	reqLogger.Info("Reconciling PostgresUser")
	before := instance.DeepCopy()

	// Deletion logic
	if !instance.GetDeletionTimestamp().IsZero() {
		if instance.Status.Succeeded && instance.Status.PostgresRole != "" {
			// Initialize database name for connection with default database
			// in case postgres cr isn't here anymore
			db := r.pg.GetDefaultDatabase()
			// Search Postgres CR
			pg, err := r.getPostgresCR(instance)
			// Check if error exists and not a not found error
			if err != nil && !errors.IsNotFound(err) {
				return ctrl.Result{}, err
			}
			// Check if postgres cr is found and not in deletion state
			if pg != nil && !pg.GetDeletionTimestamp().IsZero() {
				db = instance.Status.DatabaseName
			}
			err = r.pg.DropRole(instance.Status.PostgresRole, instance.Status.PostgresGroup,
				db, reqLogger)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
		controllerutil.RemoveFinalizer(instance, "finalizer.db.movetokube.com")

		// Update CR
		err = r.Patch(ctx, instance, client.MergeFrom(before))
		return ctrl.Result{}, err
	}

	requeue := func(err error) (ctrl.Result, error) {
		reqLogger.Error(err, "Requeuing...")
		instance.Status.Succeeded = false
		updateErr := r.Status().Update(ctx, instance)
		if updateErr != nil {
			err = kerrors.NewAggregate([]error{err, updateErr})
		}
		return ctrl.Result{Requeue: true}, err
	}

	// Creation logic
	var role, login string
	password := utils.GetRandomString(15)

	if instance.Status.PostgresRole == "" {
		// We need to get the Postgres CR to get the group role name
		database, err := r.getPostgresCR(instance)
		if err != nil {
			return requeue(errors.NewInternalError(err))
		}
		// Create user role
		suffix := utils.GetRandomString(6)
		role = fmt.Sprintf("%s-%s", instance.Spec.Role, suffix)
		login, err = r.pg.CreateUserRole(role, password)
		if err != nil {
			return requeue(errors.NewInternalError(err))
		}

		// Grant group role to user role
		var groupRole string
		switch instance.Spec.Privileges {
		case "READ":
			groupRole = database.Status.Roles.Reader
		case "WRITE":
			groupRole = database.Status.Roles.Writer
		default:
			groupRole = database.Status.Roles.Owner
		}

		err = r.pg.GrantRole(groupRole, role)
		if err != nil {
			return requeue(errors.NewInternalError(err))
		}

		// Alter default set role to group role
		// This is so that objects created by user gets owned by group role
		err = r.pg.AlterDefaultLoginRole(role, groupRole)
		if err != nil {
			return requeue(errors.NewInternalError(err))
		}
		instance.Status.PostgresRole = role
		instance.Status.PostgresGroup = groupRole
		instance.Status.PostgresLogin = login
		instance.Status.DatabaseName = database.Status.DbName

		err = r.Status().Update(ctx, instance)
		if err != nil {
			reqLogger.Error(err, "Error updating PostgresUser status")
			return requeue(err)
		}
	} else {
		role = instance.Status.PostgresRole
		login = instance.Status.PostgresLogin
	}

	err = r.addOwnerRefAndFinalizer(instance)
	if err != nil {
		return requeue(err)
	}

	secret := r.newSecretForCR(instance, role, password, login)

	// Set PostgresUser instance as the owner and controller
	if err := controllerutil.SetControllerReference(instance, secret, r.scheme); err != nil {
		reqLogger.Error(err, "setting controller reference")
		return requeue(err)
	}

	// Check if this Secret already exists
	found := &v1.Secret{}
	err = r.Get(ctx, types.NamespacedName{Name: secret.Name, Namespace: secret.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		// if role is already created, update password
		if instance.Status.Succeeded {
			err := r.pg.UpdatePassword(role, password)
			if err != nil {
				return requeue(errors.NewInternalError(err))
			}
		}
		reqLogger.Info("Creating secret", "Secret.Namespace", secret.Namespace, "Secret.Name", secret.Name)
		err = r.Create(ctx, secret)
		if err != nil {
			return requeue(err)
		}

		// Secret created successfully - don't requeue
		instance.Status.Succeeded = true
		err := r.Status().Update(ctx, instance)
		return ctrl.Result{}, err
	} else if err != nil {
		return requeue(err)
	}

	reqLogger.Info("Nothing to do", "CR.Namespace", instance.Namespace, "CR.Name", instance.Name)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PostgresUserReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dbv1alpha1.PostgresUser{}).
		Complete(r)
}

func (r *PostgresUserReconciler) newSecretForCR(cr *dbv1alpha1.PostgresUser, role, password, login string) *v1.Secret {
	pgUserUrl := fmt.Sprintf("postgresql://%s:%s@%s/%s", role, password, r.pg.GetHost(), cr.Status.DatabaseName)
	pgJDBCUrl := fmt.Sprintf("jdbc:postgresql://%s/%s", r.pg.GetHost(), cr.Status.DatabaseName)
	pgDotnetUrl := fmt.Sprintf("User ID=%s;Password=%s;Host=%s;Port=5432;Database=%s;", role, password, r.pg.GetHost(), cr.Status.DatabaseName)
	labels := map[string]string{
		"app": cr.Name,
	}
	annotations := cr.Spec.Annotations
	name := fmt.Sprintf("%s-%s", cr.Spec.SecretName, cr.Name)
	if r.keepSecretName {
		name = cr.Spec.SecretName
	}

	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   cr.Namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Data: map[string][]byte{
			"POSTGRES_URL":        []byte(pgUserUrl),
			"POSTGRES_JDBC_URL":   []byte(pgJDBCUrl),
			"POSTGRES_DOTNET_URL": []byte(pgDotnetUrl),
			"HOST":                []byte(r.pg.GetHost()),
			"DATABASE_NAME":       []byte(cr.Status.DatabaseName),
			"ROLE":                []byte(role),
			"PASSWORD":            []byte(password),
			"LOGIN":               []byte(login),
		},
	}
}

func (r *PostgresUserReconciler) getPostgresCR(instance *dbv1alpha1.PostgresUser) (*dbv1alpha1.Postgres, error) {
	database := dbv1alpha1.Postgres{}
	err := r.Get(context.TODO(),
		types.NamespacedName{Namespace: instance.Namespace, Name: instance.Spec.Database}, &database)
	if err != nil {
		return nil, err
	}
	if !utils.MatchesInstanceAnnotation(database.Annotations, r.instanceFilter) {
		err = fmt.Errorf("database \"%s\" is not managed by this operator", database.Name)
		return nil, err
	}
	if !database.Status.Succeeded {
		err = fmt.Errorf("database \"%s\" is not ready", database.Name)
		return nil, err
	}
	if database.Status.DbName == "" {
		return nil, fmt.Errorf("resource \"%s\" does not have a dbname", instance.Name)
	}
	return &database, nil
}

func (r *PostgresUserReconciler) addOwnerRefAndFinalizer(instance *dbv1alpha1.PostgresUser) error {
	// Search postgres database CR
	pg, err := r.getPostgresCR(instance)
	if err != nil {
		return err
	}
	orgInstance := instance.DeepCopy()
	updateNeeded := false
	if controllerutil.AddFinalizer(instance, "finalizer.db.movetokube.com") {
		updateNeeded = true
	}
	// Update owners
	oldOwnerCount := len(instance.OwnerReferences)
	err = controllerutil.SetOwnerReference(pg, instance, r.scheme)
	if err != nil {
		return err
	}
	// TODO: check for updated owner
	if oldOwnerCount != len(instance.OwnerReferences) {
		updateNeeded = true
	}
	// Update CR
	if updateNeeded {
		err = r.Patch(context.TODO(), instance, client.MergeFrom(orgInstance))
	}
	return err
}
