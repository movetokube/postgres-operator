package postgres

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/lib/pq"
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

var (
	getConnection = GetConnection
	openSQL       = sql.Open
	pingDB        = func(db *sql.DB) error { return db.Ping() }
)

func NewPG(cfg *config.Cfg, logger logr.Logger) (PG, error) {
	db, err := getConnection(
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

	return postgres, nil
}

func (c *pg) GetUser() string {
	return c.user
}

func (c *pg) GetDefaultDatabase() string {
	return c.defaultDatabase
}

func GetConnection(user, password, host, database, uri_args string) (*sql.DB, error) {
	db, err := openSQL("postgres", fmt.Sprintf("postgresql://%s:%s@%s/%s?%s", user, password, host, database, uri_args))
	if err != nil {
		return nil, err
	}
	return db, pingDB(db)
}

func (c *pg) execute(query string) error {
	_, err := c.db.Exec(query)
	return err
}

func isPgError(err error, code string) bool {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		return string(pqErr.Code) == code
	}
	return false
}
