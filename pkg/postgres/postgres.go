package postgres

import (
	"database/sql"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/movetokube/postgres-operator/pkg/config"
)

type PG interface {
	CreateDB(dbname, username string) error
	CreateSchema(db, role, schema string) error
	CreateExtension(db, extension string) error
	CreateGroupRole(role string) error
	RenameGroupRole(currentRole, newRole string) error
	CreateUserRole(role, password string) (string, error)
	UpdatePassword(role, password string) error
	GrantRole(role, grantee string) error
	AlterDatabaseOwner(dbName, owner string) error
	ReassignDatabaseOwner(dbName, currentOwner, newOwner string) error
	SetSchemaPrivileges(schemaPrivileges PostgresSchemaPrivileges) error
	RevokeRole(role, revoked string) error
	AlterDefaultLoginRole(role, setRole string) error
	DropDatabase(db string) error
	DropRole(role, newOwner, database string) error
	GetUser() string
	GetDefaultDatabase() string
}

type pg struct {
	db              *sql.DB
	log             logr.Logger
	host            string
	user            string
	pass            string
	args            string
	defaultDatabase string
}

type PostgresSchemaPrivileges struct {
	DB            string
	Role          string
	Schema        string
	Privs         string
	SequencePrivs string
	FunctionPrivs string
	CreateSchema  bool
}

func NewPG(cfg *config.Cfg, logger logr.Logger) (PG, error) {
	db, err := GetConnection(
		cfg.PostgresUser,
		cfg.PostgresPass,
		cfg.PostgresHost,
		cfg.PostgresDefaultDb,
		cfg.PostgresUriArgs)
	if err != nil {
		return nil, err
	}
	logger.V(1).Info("connected to postgres server")
	postgres := &pg{
		db:              db,
		log:             logger,
		host:            cfg.PostgresHost,
		user:            cfg.PostgresUser,
		pass:            cfg.PostgresPass,
		args:            cfg.PostgresUriArgs,
		defaultDatabase: cfg.PostgresDefaultDb,
	}

	switch cfg.CloudProvider {
	case config.CloudProviderAWS:
		logger.Info("Using AWS wrapper")
		return newAWSPG(postgres), nil
	case config.CloudProviderAzure:
		logger.Info("Using Azure wrapper")
		return newAzurePG(postgres), nil
	case config.CloudProviderGCP:
		logger.Info("Using GCP wrapper")
		return newGCPPG(postgres), nil
	default:
		logger.Info("Using default postgres implementation")
		return postgres, nil
	}
}

func (c *pg) GetUser() string {
	return c.user
}

func (c *pg) GetDefaultDatabase() string {
	return c.defaultDatabase
}

func GetConnection(user, password, host, database, uriArgs string) (*sql.DB, error) {
	db, err := sql.Open("postgres", fmt.Sprintf("postgresql://%s:%s@%s/%s?%s", user, password, host, database, uriArgs))
	if err != nil {
		return nil, err
	}
	err = db.Ping()
	return db, err
}
