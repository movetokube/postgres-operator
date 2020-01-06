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
}

type pg struct {
	db   *sql.DB
	log  logr.Logger
	host string
	user string
	pass string
	args string
}

func NewPG(host, user, password, uri_args, default_database, cloud_type string, logger logr.Logger) (PG, error) {
	postgres := &pg{
		db:   GetConnection(user, password, host, default_database, uri_args, logger),
		log:  logger,
		host: host,
		user: user,
		pass: password,
		args: uri_args,
	}

	switch cloud_type {
	case "AWS":
		return newAWSPG(postgres), nil
	case "Azure":
		return newAzurePG(postgres), nil
	default:
		return postgres, nil
	}
}

func (c *pg) GetUser() string {
	return c.user
}

func GetConnection(user, password, host, database, uri_args string, logger logr.Logger) *sql.DB {
	db, err := sql.Open("postgres", fmt.Sprintf("postgresql://%s:%s@%s/%s?%s", user, password, host, database, uri_args))
	if err != nil {
		log.Fatal(err)
	}
	err = db.Ping()
	if err != nil {
		log.Fatalf("failed to connect to PostgreSQL server: %s", err.Error())
	}
	logger.Info("connected to postgres server")
	return db
}
