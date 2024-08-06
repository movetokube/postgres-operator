package postgresuser

import (
	"bytes"
	"context"
	goerr "errors"
	"fmt"
	"text/template"

	"github.com/movetokube/postgres-operator/pkg/config"

	"github.com/go-logr/logr"
	dbv1alpha1 "github.com/movetokube/postgres-operator/pkg/apis/db/v1alpha1"
	"github.com/movetokube/postgres-operator/pkg/postgres"
	"github.com/movetokube/postgres-operator/pkg/utils"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_postgresuser")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new PostgresUser Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	c := config.Get()
	pg, err := postgres.NewPG(c.PostgresHost, c.PostgresUser, c.PostgresPass, c.PostgresUriArgs, c.PostgresDefaultDb, c.CloudProvider, log.WithName("postgresuser"))
	if err != nil {
		return nil
	}

	return &ReconcilePostgresUser{
		client:         mgr.GetClient(),
		scheme:         mgr.GetScheme(),
		pg:             pg,
		pgHost:         c.PostgresHost,
		instanceFilter: c.AnnotationFilter,
		keepSecretName: c.KeepSecretName,
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	if r == nil {
		return errors.NewInternalError(goerr.New("failed to get reconciler"))
	}
	// Create a new controller
	c, err := controller.New("postgresuser-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource PostgresUser
	err = c.Watch(&source.Kind{Type: &dbv1alpha1.PostgresUser{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to the generated secret
	err = c.Watch(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &dbv1alpha1.PostgresUser{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcilePostgresUser implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcilePostgresUser{}

// ReconcilePostgresUser reconciles a PostgresUser object
type ReconcilePostgresUser struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client         client.Client
	scheme         *runtime.Scheme
	pg             postgres.PG
	pgHost         string
	instanceFilter string
	keepSecretName bool // use secret name as defined in PostgresUserSpec
}

// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcilePostgresUser) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling PostgresUser")

	// Fetch the PostgresUser instance
	instance := &dbv1alpha1.PostgresUser{}
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

	// Deletion logic
	if instance.GetDeletionTimestamp() != nil {
		if instance.Status.Succeeded && instance.Status.PostgresRole != "" {
			// Initialize database name for connection with default database
			// in case postgres cr isn't here anymore
			db := r.pg.GetDefaultDatabase()
			// Search Postgres CR
			postgres, err := r.getPostgresCR(instance)
			// Check if error exists and not a not found error
			if err != nil && !errors.IsNotFound(err) {
				return reconcile.Result{}, err
			}
			// Check if postgres cr is found and not in deletion state
			if postgres != nil && !postgres.GetDeletionTimestamp().IsZero() {
				db = instance.Status.DatabaseName
			}
			err = r.pg.DropRole(instance.Status.PostgresRole, instance.Status.PostgresGroup,
				db, reqLogger)
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

	// Creation logic
	var role, login string
	password, err := utils.GetSecureRandomString(15)

	if err != nil {
		return r.requeue(instance, err)
	}

	if instance.Status.PostgresRole == "" {
		// We need to get the Postgres CR to get the group role name
		database, err := r.getPostgresCR(instance)
		if err != nil {
			return r.requeue(instance, errors.NewInternalError(err))
		}
		// Create user role
		suffix := utils.GetRandomString(6)
		role = fmt.Sprintf("%s-%s", instance.Spec.Role, suffix)
		login, err = r.pg.CreateUserRole(role, password, instance.spec.IamAuthentication)
		if err != nil {
			return r.requeue(instance, errors.NewInternalError(err))
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
			return r.requeue(instance, errors.NewInternalError(err))
		}

		// Alter default set role to group role
		// This is so that objects created by user gets owned by group role
		err = r.pg.AlterDefaultLoginRole(role, groupRole)
		if err != nil {
			return r.requeue(instance, errors.NewInternalError(err))
		}

		instance.Status.PostgresRole = role
		instance.Status.PostgresGroup = groupRole
		instance.Status.PostgresLogin = login
		instance.Status.DatabaseName = database.Spec.Database
		err = r.client.Status().Update(context.TODO(), instance)
		if err != nil {
			return r.requeue(instance, err)
		}
	} else {
		role = instance.Status.PostgresRole
		login = instance.Status.PostgresLogin
	}

	err = r.addFinalizer(reqLogger, instance)
	if err != nil {
		return r.requeue(instance, err)
	}
	err = r.addOwnerRef(reqLogger, instance)
	if err != nil {
		return r.requeue(instance, err)
	}

	secret, err := r.newSecretForCR(instance, role, password, login)
	if err != nil {
		return r.requeue(instance, err)
	}

	// Set PostgresUser instance as the owner and controller
	if err := controllerutil.SetControllerReference(instance, secret, r.scheme); err != nil {
		return r.requeue(instance, err)
	}

	// Check if this Secret already exists
	found := &corev1.Secret{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: secret.Name, Namespace: secret.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		// if role is already created, update password
		if instance.Status.Succeeded {
			err := r.pg.UpdatePassword(role, password)
			if err != nil {
				return r.requeue(instance, err)
			}
		}
		reqLogger.Info("Creating secret", "Secret.Namespace", secret.Namespace, "Secret.Name", secret.Name)
		err = r.client.Create(context.TODO(), secret)
		if err != nil {
			return reconcile.Result{}, err
		}

		// Secret created successfully - don't requeue
		return r.finish(instance)
	} else if err != nil {
		return r.requeue(instance, err)
	}

	reqLogger.Info("reconciler done", "CR.Namespace", instance.Namespace, "CR.Name", instance.Name)
	return reconcile.Result{}, nil
}

func (r *ReconcilePostgresUser) addFinalizer(reqLogger logr.Logger, m *dbv1alpha1.PostgresUser) error {
	if len(m.GetFinalizers()) < 1 && m.GetDeletionTimestamp() == nil {
		reqLogger.Info("adding Finalizer for Postgres")
		m.SetFinalizers([]string{"finalizer.db.movetokube.com"})

		// Update CR
		err := r.client.Update(context.TODO(), m)
		if err != nil {
			reqLogger.Error(err, "failed to update PosgresUser with finalizer")
			return err
		}
	}
	return nil
}

func (r *ReconcilePostgresUser) newSecretForCR(cr *dbv1alpha1.PostgresUser, role, password, login string) (*corev1.Secret, error) {
	pgUserUrl := fmt.Sprintf("postgresql://%s:%s@%s/%s", role, password, r.pgHost, cr.Status.DatabaseName)
	pgJDBCUrl := fmt.Sprintf("jdbc:postgresql://%s/%s", r.pgHost, cr.Status.DatabaseName)
	pgDotnetUrl := fmt.Sprintf("User ID=%s;Password=%s;Host=%s;Port=5432;Database=%s;", role, password, r.pgHost, cr.Status.DatabaseName)
	labels := map[string]string{
		"app": cr.Name,
	}
	annotations := cr.Spec.Annotations
	name := fmt.Sprintf("%s-%s", cr.Spec.SecretName, cr.Name)
	if r.keepSecretName {
		name = cr.Spec.SecretName
	}

	templateData, err := renderTemplate(cr.Spec.SecretTemplate, templateContext{
		Role:     role,
		Host:     r.pgHost,
		Database: cr.Status.DatabaseName,
		Password: password,
	})
	if err != nil {
		return nil, fmt.Errorf("render templated keys: %w", err)
	}

	data := map[string][]byte{
		"POSTGRES_URL":        []byte(pgUserUrl),
		"POSTGRES_JDBC_URL":   []byte(pgJDBCUrl),
		"POSTGRES_DOTNET_URL": []byte(pgDotnetUrl),
		"HOST":                []byte(r.pgHost),
		"DATABASE_NAME":       []byte(cr.Status.DatabaseName),
		"ROLE":                []byte(role),
		"PASSWORD":            []byte(password),
		"LOGIN":               []byte(login),
	}
	// templates may override standard keys
	for k, v := range templateData {
		data[k] = v
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   cr.Namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Data: data,
	}, nil
}

func (r *ReconcilePostgresUser) requeue(cr *dbv1alpha1.PostgresUser, reason error) (reconcile.Result, error) {
	cr.Status.Succeeded = false
	err := r.client.Status().Update(context.TODO(), cr)
	if err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, reason
}

func (r *ReconcilePostgresUser) finish(cr *dbv1alpha1.PostgresUser) (reconcile.Result, error) {
	cr.Status.Succeeded = true
	err := r.client.Status().Update(context.TODO(), cr)
	if err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

func (r *ReconcilePostgresUser) getPostgresCR(instance *dbv1alpha1.PostgresUser) (*dbv1alpha1.Postgres, error) {
	database := dbv1alpha1.Postgres{}
	err := r.client.Get(context.TODO(),
		types.NamespacedName{Namespace: instance.Namespace, Name: instance.Spec.Database}, &database)
	if !utils.MatchesInstanceAnnotation(database.Annotations, r.instanceFilter) {
		err = fmt.Errorf("database \"%s\" is not managed by this operator", database.Name)
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	if !database.Status.Succeeded {
		err = fmt.Errorf("database \"%s\" is not ready", database.Name)
		return nil, err
	}
	return &database, nil
}

func (r *ReconcilePostgresUser) addOwnerRef(reqLogger logr.Logger, instance *dbv1alpha1.PostgresUser) error {
	// Search postgres database CR
	pg, err := r.getPostgresCR(instance)
	if err != nil {
		return err
	}
	// Update owners
	err = controllerutil.SetControllerReference(pg, instance, r.scheme)
	if err != nil {
		return err
	}
	// Update CR
	err = r.client.Update(context.TODO(), instance)
	return err
}

type templateContext struct {
	Host     string
	Role     string
	Database string
	Password string
}

func renderTemplate(data map[string]string, tc templateContext) (map[string][]byte, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var out = make(map[string][]byte, len(data))
	for key, templ := range data {
		parsed, err := template.New("").Parse(templ)
		if err != nil {
			return nil, fmt.Errorf("parse template %q: %w", key, err)
		}
		var content bytes.Buffer
		if err := parsed.Execute(&content, tc); err != nil {
			return nil, fmt.Errorf("execute template %q: %w", key, err)
		}
		out[key] = content.Bytes()
	}
	return out, nil
}
