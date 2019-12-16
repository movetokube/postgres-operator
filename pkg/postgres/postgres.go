package postgres

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/go-logr/logr"
)

type PG interface {
	Connect() error
	CreateDB(dbname, username string) error
	CreateSchema(db, role, schema string, logger logr.Logger) error
	CreateGroupRole(role string) error
	CreateUserRole(role, password string) error
	UpdatePassword(role, password string) error
	GrantRole(role, grantee string) error
	SetSchemaPrivileges(db, creator, role, schema, privs string, logger logr.Logger) error
	RevokeRole(role, revoked string) error
	AlterDefaultLoginRole(role, setRole string) error
	DropDatabase(db string, logger logr.Logger) error
	DropRole(role, newOwner, database string, logger logr.Logger) error
	GetUser() string
	GetLoginForRole(role string) string
}

type pg struct {
	db   *sql.DB
	log  logr.Logger
	host string
	user string
	pass string
	args string
}

func NewPG(host, user, password, uri_args, cloud_type string, logger logr.Logger) (PG, error) {
	var postgres PG
	postgres = &pg{
		log:  logger,
		host: host,
		user: user,
		pass: password,
		args: uri_args,
	}

	switch cloud_type {
	case "AWS":
		postgres = newAWSPG(postgres.(*pg))
	case "Azure":
		postgres = newAzurePG(postgres.(*pg))
	}
	postgres.Connect()
	return postgres, nil
}

func (c *pg) GetUser() string {
	return c.user
}

func (c *pg) GetLoginForRole(role string) string {
	return role
}

func (c *pg) Connect() error {
	c.db = GetConnection(c.user, c.pass, c.host, "", c.args, c.log)
	return nil
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
