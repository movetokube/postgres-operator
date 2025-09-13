package controller

import (
	"context"
	"fmt"
	"maps"
	"net"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	dbv1alpha1 "github.com/movetokube/postgres-operator/api/v1alpha1"
	"github.com/movetokube/postgres-operator/pkg/config"
	"github.com/movetokube/postgres-operator/pkg/postgres"
	"github.com/movetokube/postgres-operator/pkg/utils"
)

// PostgresUserReconciler reconciles a PostgresUser object
type PostgresUserReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	pg             postgres.PG
	pgHost         string
	pgUriArgs      string
	instanceFilter string
	keepSecretName bool // use secret name as defined in PostgresUserSpec
}

// NewPostgresUserReconciler returns a new reconcile.Reconciler
func NewPostgresUserReconciler(mgr manager.Manager, cfg *config.Cfg, pg postgres.PG) *PostgresUserReconciler {
	return &PostgresUserReconciler{
		Client:         mgr.GetClient(),
		Scheme:         mgr.GetScheme(),
		pg:             pg,
		pgHost:         cfg.PostgresHost,
		pgUriArgs:      cfg.PostgresUriArgs,
		instanceFilter: cfg.AnnotationFilter,
		keepSecretName: cfg.KeepSecretName,
	}
}

// +kubebuilder:rbac:groups=db.movetokube.com,resources=postgresusers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=db.movetokube.com,resources=postgresusers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=db.movetokube.com,resources=postgresusers/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/reconcile
func (r *PostgresUserReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	reqLogger := log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	reqLogger.Info("Reconciling PostgresUser")

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

	// Deletion logic
	if instance.GetDeletionTimestamp() != nil {
		if instance.Status.Succeeded && instance.Status.PostgresRole != "" {
			// Build database -> groupRole mapping from status (multi-db aware)
			ownerByDB := map[string]string{}
			if len(instance.Status.Grants) > 0 {
				for _, g := range instance.Status.Grants {
					// Skip empty values just in case
					if g.DatabaseName != "" && g.PostgresGroup != "" {
						ownerByDB[g.DatabaseName] = g.PostgresGroup
					}
				}
			}
			// Backward compatibility: single DB fields
			if len(ownerByDB) == 0 && instance.Status.DatabaseName != "" && instance.Status.PostgresGroup != "" {
				ownerByDB[instance.Status.DatabaseName] = instance.Status.PostgresGroup
			}
			// If still empty, fallback to default database to allow DropRole to proceed
			if len(ownerByDB) == 0 {
				ownerByDB[r.pg.GetDefaultDatabase()] = instance.Status.PostgresGroup
			}

			type dropper interface {
				DropRoleMulti(role string, ownerByDB map[string]string, logger logr.Logger) error
			}
			if dr, ok := r.pg.(dropper); ok {
				if err := dr.DropRoleMulti(instance.Status.PostgresRole, ownerByDB, reqLogger); err != nil {
					return ctrl.Result{}, err
				}
			} else {
				// Fallback: try single-db drop using the first entry
				for dbName, group := range ownerByDB {
					if err := r.pg.DropRole(instance.Status.PostgresRole, group, dbName, reqLogger); err != nil {
						return ctrl.Result{}, err
					}
					break
				}
			}
		}
		controllerutil.RemoveFinalizer(instance, "finalizer.db.movetokube.com")

		// Update CR
		err = r.Update(ctx, instance)
		if err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Creation logic
	var role, login string
	password, err := utils.GetSecureRandomString(15)

	if err != nil {
		return r.requeue(ctx, instance, err)
	}

	if instance.Status.PostgresRole == "" {
		// Resolve desired databases and privileges (supports both legacy and new spec)
		var desired []dbv1alpha1.PostgresUserDatabaseRef
		if len(instance.Spec.Databases) > 0 {
			desired = instance.Spec.Databases
		} else if instance.Spec.Database != "" {
			desired = []dbv1alpha1.PostgresUserDatabaseRef{{
				Name:       instance.Spec.Database,
				Privileges: instance.Spec.Privileges,
			}}
		} else {
			return r.requeue(ctx, instance, fmt.Errorf("no databases specified in spec"))
		}

		// Fetch all Postgres CRs and compute group roles
		type dbGrant struct{ dbName, groupRole, dbActual string }
		grants := make([]dbGrant, 0, len(desired))
		for _, ref := range desired {
			pgcr, err := r.getPostgresByName(ctx, instance.Namespace, ref.Name)
			if err != nil {
				return r.requeue(ctx, instance, errors.NewInternalError(err))
			}
			var group string
			switch ref.Privileges {
			case "READ":
				group = pgcr.Status.Roles.Reader
			case "WRITE":
				group = pgcr.Status.Roles.Writer
			default:
				group = pgcr.Status.Roles.Owner
			}
			grants = append(grants, dbGrant{dbName: pgcr.Spec.Database, groupRole: group, dbActual: pgcr.Spec.Database})
		}

		// Create user role
		suffix := utils.GetRandomString(6)
		role = fmt.Sprintf("%s-%s", instance.Spec.Role, suffix)
		login, err = r.pg.CreateUserRole(role, password)
		if err != nil {
			return r.requeue(ctx, instance, errors.NewInternalError(err))
		}

		// Grant group roles to user role for each database
		for _, g := range grants {
			if err := r.pg.GrantRole(g.groupRole, role); err != nil {
				return r.requeue(ctx, instance, errors.NewInternalError(err))
			}
		}

		// Set default login role only when exactly one database is granted.
		// For multi-database users, leaving the default role unset ensures the login role
		// inherits privileges from all granted group roles simultaneously.
		if len(grants) == 1 {
			if err := r.pg.AlterDefaultLoginRole(role, grants[0].groupRole); err != nil {
				return r.requeue(ctx, instance, errors.NewInternalError(err))
			}
		}

		// Update status (store first db fields for backwards compatibility)
		instance.Status.PostgresRole = role
		instance.Status.PostgresLogin = login
		if len(grants) > 0 {
			instance.Status.PostgresGroup = grants[0].groupRole
			instance.Status.DatabaseName = grants[0].dbActual
		}
		// Fill detailed grants
		instance.Status.Grants = nil
		for _, g := range grants {
			instance.Status.Grants = append(instance.Status.Grants, dbv1alpha1.PostgresUserDatabaseGrant{
				DatabaseName:  g.dbName,
				PostgresGroup: g.groupRole,
			})
		}
		err = r.Status().Update(ctx, instance)
		if err != nil {
			return r.requeue(ctx, instance, err)
		}
	} else {
		role = instance.Status.PostgresRole
		login = instance.Status.PostgresLogin
	}

	err = r.addFinalizer(ctx, reqLogger, instance)
	if err != nil {
		return r.requeue(ctx, instance, err)
	}
	err = r.addOwnerRef(ctx, reqLogger, instance)
	if err != nil {
		return r.requeue(ctx, instance, err)
	}

	secret, err := r.newSecretForCR(reqLogger, instance, role, password, login)
	if err != nil {
		return r.requeue(ctx, instance, err)
	}

	// Set PostgresUser instance as the owner and controller
	if err := controllerutil.SetControllerReference(instance, secret, r.Scheme); err != nil {
		return r.requeue(ctx, instance, err)
	}

	// Check if this Secret already exists
	found := &corev1.Secret{}
	err = r.Get(ctx, types.NamespacedName{Name: secret.Name, Namespace: secret.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		// if role is already created, update password
		if instance.Status.Succeeded {
			err := r.pg.UpdatePassword(role, password)
			if err != nil {
				return r.requeue(ctx, instance, err)
			}
		}
		reqLogger.Info("Creating secret", "Secret.Namespace", secret.Namespace, "Secret.Name", secret.Name)
		err = r.Create(ctx, secret)
		if err != nil {
			return ctrl.Result{}, err
		}

		// Secret created successfully - don't requeue
		return r.finish(ctx, instance)
	} else if err != nil {
		return r.requeue(ctx, instance, err)
	}

	reqLogger.Info("reconciler done", "CR.Namespace", instance.Namespace, "CR.Name", instance.Name)
	return ctrl.Result{}, nil
}

func (r *PostgresUserReconciler) getPostgresCR(ctx context.Context, instance *dbv1alpha1.PostgresUser) (*dbv1alpha1.Postgres, error) {
	database := dbv1alpha1.Postgres{}
	err := r.Get(ctx,
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
	return &database, nil
}

func (r *PostgresUserReconciler) getPostgresByName(ctx context.Context, namespace, name string) (*dbv1alpha1.Postgres, error) {
	database := dbv1alpha1.Postgres{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &database); err != nil {
		return nil, err
	}
	if !utils.MatchesInstanceAnnotation(database.Annotations, r.instanceFilter) {
		return nil, fmt.Errorf("database \"%s\" is not managed by this operator", database.Name)
	}
	if !database.Status.Succeeded {
		return nil, fmt.Errorf("database \"%s\" is not ready", database.Name)
	}
	return &database, nil
}

func (r *PostgresUserReconciler) newSecretForCR(reqLogger logr.Logger, cr *dbv1alpha1.PostgresUser, role, password, login string) (*corev1.Secret, error) {
	hostname, port, err := net.SplitHostPort(r.pgHost)
	if err != nil {
		hostname = r.pgHost
		port = "5432"
		reqLogger.Error(err, fmt.Sprintf("failed to parse host and port from: '%s', using default port 5432", r.pgHost))
	}

	pgUserUrl := fmt.Sprintf("postgresql://%s:%s@%s/%s", role, password, r.pgHost, cr.Status.DatabaseName)
	pgJDBCUrl := fmt.Sprintf("jdbc:postgresql://%s/%s", r.pgHost, cr.Status.DatabaseName)
	pgDotnetUrl := fmt.Sprintf("User ID=%s;Password=%s;Host=%s;Port=%s;Database=%s;", role, password, hostname, port, cr.Status.DatabaseName)
	labels := map[string]string{
		"app": cr.Name,
	}
	// Merge in user-defined secret labels
	maps.Copy(labels, cr.Spec.Labels)

	annotations := cr.Spec.Annotations
	name := fmt.Sprintf("%s-%s", cr.Spec.SecretName, cr.Name)
	if r.keepSecretName {
		name = cr.Spec.SecretName
	}

	templateData, err := utils.RenderTemplate(cr.Spec.SecretTemplate, utils.TemplateContext{
		Role:     role,
		Host:     r.pgHost,
		UriArgs:  r.pgUriArgs,
		Database: cr.Status.DatabaseName,
		Password: password,
		Hostname: hostname,
		Port:     port,
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
		"URI_ARGS":            []byte(r.pgUriArgs),
		"ROLE":                []byte(role),
		"PASSWORD":            []byte(password),
		"LOGIN":               []byte(login),
		"PORT":                []byte(port),
		"HOSTNAME":            []byte(hostname),
	}
	// templates may override standard keys
	if len(templateData) > 0 {
		maps.Copy(data, templateData)
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

func (r *PostgresUserReconciler) addFinalizer(ctx context.Context, reqLogger logr.Logger, m *dbv1alpha1.PostgresUser) error {
	if len(m.GetFinalizers()) < 1 && m.GetDeletionTimestamp() == nil {
		reqLogger.Info("adding Finalizer for Postgres")
		m.SetFinalizers([]string{"finalizer.db.movetokube.com"})

		// Update CR
		err := r.Update(ctx, m)
		if err != nil {
			reqLogger.Error(err, "failed to update PosgresUser with finalizer")
			return err
		}
	}
	return nil
}

func (r *PostgresUserReconciler) addOwnerRef(ctx context.Context, _ logr.Logger, instance *dbv1alpha1.PostgresUser) error {
	// Search postgres database CR (use first referenced DB)
	var pg *dbv1alpha1.Postgres
	var err error
	if len(instance.Spec.Databases) > 0 {
		pg, err = r.getPostgresByName(ctx, instance.Namespace, instance.Spec.Databases[0].Name)
	} else {
		pg, err = r.getPostgresCR(ctx, instance)
	}
	if err != nil {
		return err
	}
	// Update owners
	err = controllerutil.SetControllerReference(pg, instance, r.Scheme)
	if err != nil {
		return err
	}
	// Update CR
	err = r.Update(ctx, instance)
	return err
}

func (r *PostgresUserReconciler) requeue(ctx context.Context, cr *dbv1alpha1.PostgresUser, reason error) (ctrl.Result, error) {
	cr.Status.Succeeded = false
	err := r.Status().Update(ctx, cr)
	if err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, reason
}

func (r *PostgresUserReconciler) finish(ctx context.Context, cr *dbv1alpha1.PostgresUser) (ctrl.Result, error) {
	cr.Status.Succeeded = true
	err := r.Status().Update(ctx, cr)
	if err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PostgresUserReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dbv1alpha1.PostgresUser{}).
		Complete(r)
}
