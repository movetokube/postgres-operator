package postgres

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/go-logr/logr"
)

type PG interface {
	CreateDB(dbname, username string) error
	CreateSchema(db, role, schema string, logger logr.Logger) error
	CreateExtension(db, extension string, logger logr.Logger) error
	CreateGroupRole(role string) error
	CreateUserRole(role, password string) (string, error)
	UpdatePassword(role, password string) error
	GrantRole(role, grantee string) error
	SetSchemaPrivileges(db, creator, role, schema, privs string, logger logr.Logger) error
	RevokeRole(role, revoked string) error
	AlterDefaultLoginRole(role, setRole string) error
	DropDatabase(db string, logger logr.Logger) error
	DropRole(role, newOwner, database string, logger logr.Logger) error
	GetUser() string
	GetDefaultDatabase() string
}

type pg struct {
	db               *sql.DB
	log              logr.Logger
	host             string
	user             string
	pass             string
	args             string
	default_database string
}

func NewPG(host, user, password, uri_args, default_database, cloud_type string, logger logr.Logger) (PG, error) {
	db, err := GetConnection(user, password, host, default_database, uri_args, logger)
	if err != nil {
		log.Fatalf("failed to connect to PostgreSQL server: %s", err.Error())
	}
	logger.Info("connected to postgres server")
	postgres := &pg{
		db:               db,
		log:              logger,
		host:             host,
		user:             user,
		pass:             password,
		args:             uri_args,
		default_database: default_database,
	}

	switch cloud_type {
	case "AWS":
		logger.Info("Using AWS wrapper")
		return newAWSPG(postgres), nil
	case "Azure":
		logger.Info("Using Azure wrapper")
		return newAzurePG(postgres), nil
	case "GCP":
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
	return c.default_database
}

func GetConnection(user, password, host, database, uri_args string, logger logr.Logger) (*sql.DB, error) {
	db, err := sql.Open("postgres", fmt.Sprintf("postgresql://%s:%s@%s/%s?%s", user, password, host, database, uri_args))
	if err != nil {
		log.Fatal(err)
	}
	err = db.Ping()
	return db, err
}
