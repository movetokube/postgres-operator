package postgres

import (
	"database/sql"
	"fmt"
	"github.com/go-logr/logr"
	"log"
)
import "github.com/lib/pq"

type PG interface {
	CreateDB(dbname, username string) error
	CreateGroupRole(role string) error
	CreateUserRole(role, password string) error
	UpdatePassword(role, password string) error
	GrantRole(role, grantee string) error
	RevokeRole(role, revoked string) error
	AlterDefaultLoginRole(role, setRole string) error
	DropDatabase(database string, logger logr.Logger) error
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

func NewPG(host, user, password, uri_args string, logger logr.Logger) (*pg, error) {
	return &pg{
		db:   GetConnection(user, password, host, "", uri_args, logger),
		log:  logger,
		host: host,
		user: user,
		pass: password,
		args: uri_args,
	}, nil
}

func (c *pg) CreateDB(dbname, role string) error {
	_, err := c.db.Exec(fmt.Sprintf(CREATE_DB, dbname))
	if err != nil {
		// eat DUPLICATE DATABASE ERROR
		if err.(*pq.Error).Code != "42P04" {
			return err
		}
	}

	_, err = c.db.Exec(fmt.Sprintf(ALTER_DB_OWNER, dbname, role))
	if err != nil {
		return err
	}
	return nil
}

func (c *pg) CreateGroupRole(role string) error {
	// Error code 42710 is duplicate_object (role already exists)
	_, err := c.db.Exec(fmt.Sprintf(CREATE_GROUP_ROLE, role))
	if err != nil && err.(*pq.Error).Code != "42710" {
		return err
	}
	return nil
}

func (c *pg) CreateUserRole(role, password string) error {
	_, err := c.db.Exec(fmt.Sprintf(CREATE_USER_ROLE, role, password))
	if err != nil {
		return err
	}
	return nil
}

func (c *pg) GrantRole(role, grantee string) error {
	_, err := c.db.Exec(fmt.Sprintf(GRANT_ROLE, role, grantee))
	if err != nil {
		return err
	}
	return nil
}

func (c *pg) AlterDefaultLoginRole(role, setRole string) error {
	// On AWS RDS the postgres user isn't really superuser so he doesn't have permissions
	// to ALTER USER unless he belongs to both roles
	err := c.GrantRole(role, c.user)
	if err != nil {
		return err
	}
	defer c.RevokeRole(role, c.user)
	_, err = c.db.Exec(fmt.Sprintf(ALTER_USER_SET_ROLE, role, setRole))
	if err != nil {
		return err
	}
	return nil
}

func (c *pg) RevokeRole(role, revoked string) error {
	_, err := c.db.Exec(fmt.Sprintf(REVOKE_ROLE, role, revoked))
	if err != nil {
		return err
	}
	return nil
}

func (c *pg) DropDatabase(database string, logger logr.Logger) error {
	_, err := c.db.Exec(fmt.Sprintf(DROP_DATABASE, database))
	// Error code 3D000 is returned if database doesn't exist
	if err != nil && err.(*pq.Error).Code != "3D000" {
		return err
	}

	logger.Info(fmt.Sprintf("Dropped database %s", database))

	return nil
}

func (c *pg) DropRole(role, newOwner, database string, logger logr.Logger) error {
	// On AWS RDS the postgres user isn't really superuser so he doesn't have permissions
	// to REASSIGN OWNED BY unless he belongs to both roles
	err := c.GrantRole(role, c.user)
	if err != nil && err.(*pq.Error).Code != "0LP01" {
		return err
	}
	err = c.GrantRole(newOwner, c.user)
	if err != nil && err.(*pq.Error).Code != "0LP01" {
		if err.(*pq.Error).Code == "42704" {
			// The group role does not exist, no point of granting roles
			logger.Info(fmt.Sprintf("not granting %s to %s as %s does not exist", role, newOwner, newOwner))
			return nil
		}
		return err
	}
	defer c.RevokeRole(newOwner, c.user)
	// REASSIGN OWNED BY only works if the correct database is selected
	tmpDb := GetConnection(c.user, c.pass, c.host, database, c.args, logger)
	_, err = tmpDb.Exec(fmt.Sprintf(REASIGN_OBJECTS, role, newOwner))
	defer tmpDb.Close()
	if err != nil && err.(*pq.Error).Code != "42704" {
		return err
	}

	_, err = c.db.Exec(fmt.Sprintf(DROP_ROLE, role))
	if err != nil && err.(*pq.Error).Code != "42704" {
		return err
	}
	logger.Info(fmt.Sprintf("Dropped role %s", role))
	return nil
}

func (c *pg) UpdatePassword(role, password string) error {
	_, err := c.db.Exec(fmt.Sprintf(UPDATE_PASSWORD, role, password))
	if err != nil {
		return err
	}

	return nil
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
