package postgres

import (
	"context"
	goerr "errors"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/hitman99/postgres-operator/pkg/postgres"
	"github.com/hitman99/postgres-operator/pkg/utils"
	"k8s.io/apimachinery/pkg/api/errors"
	"os"

	dbv1alpha1 "github.com/hitman99/postgres-operator/pkg/apis/db/v1alpha1"

	corev1 "k8s.io/api/core/v1"
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

var log = logf.Log.WithName("controller_postgres")

// Add creates a new Postgres Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	pgHost, found := os.LookupEnv("POSTGRES_HOST")
	if !found {
		return nil
	}
	pgUrl, found := os.LookupEnv("POSTGRES_URL")
	if !found {
		return nil
	}
	pg, err := postgres.NewPG(pgUrl, log.WithName("postgres"))
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
	// Watch for changes to the generated secret
	err = c.Watch(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &dbv1alpha1.Postgres{},
	})
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
		if instance.Status.Succeeded && instance.Status.PostgresRole != "" {
			err := r.pg.DropRole(instance.Status.PostgresRole, instance.Spec.Database)
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
	// check if user was generated already
	var (
		role string
	)
	password := utils.GetRandomString(15)

	if instance.Status.PostgresRole == "" {
		suffix := utils.GetRandomString(6)
		role = fmt.Sprintf("%s-user-%s", instance.Spec.Database, suffix)
		err = r.pg.CreateRole(role, password)
		if err != nil {
			return r.requeue(instance, errors.NewInternalError(err))
		}
		err = r.pg.CreateDB(instance.Spec.Database, role)
		if err != nil {
			return r.requeue(instance, errors.NewInternalError(err))
		}

		instance.Status.PostgresRole = role
		err = r.client.Update(context.TODO(), instance)
		if err != nil {
			return r.requeue(instance, err)
		}
	} else {
		role = instance.Status.PostgresRole
	}

	err = r.addFinalizer(reqLogger, instance)
	if err != nil {
		return r.requeue(instance, err)
	}

	secret := r.newSecretForCR(instance, role, password)

	// Set Postgres instance as the owner and controller
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
		reqLogger.Info("creating secret", "Secret.Namespace", secret.Namespace, "Secret.Name", secret.Name)
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

func (r *ReconcilePostgres) newSecretForCR(cr *dbv1alpha1.Postgres, role, password string) *corev1.Secret {
	pgUserUrl := fmt.Sprintf("postgresql://%s:%s@%s/%s", role, password, r.pgHost, cr.Spec.Database)
	labels := map[string]string{
		"app": cr.Name,
	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", cr.Spec.SecretName, cr.Name),
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Data: map[string][]byte{
			"POSTGRES_URL": []byte(pgUserUrl),
			"ROLE":         []byte(role),
			"PASSWORD":     []byte(password),
		},
	}
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
