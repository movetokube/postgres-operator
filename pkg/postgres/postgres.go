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
	CreateRole(role, password string) error
	UpdatePassword(role, password string) error
	DropRole(role, database string, logger logr.Logger) error
}

type pg struct {
	db   *sql.DB
	host string
	user string
	pass string
	args string
}

func NewPG(host, user, password, uri_args string, logger logr.Logger) (*pg, error) {
	return &pg{
		db:   GetConnection(user, password, host, "", uri_args, logger),
		host: host,
		user: user,
		pass: password,
		args: uri_args,
	}, nil
}

func (c *pg) CreateDB(dbname, username string) error {
	_, err := c.db.Exec(fmt.Sprintf(CREATE_DB, dbname))
	if err != nil {
		// eat DUPLICATE DATABASE ERROR
		if err.(*pq.Error).Code == "42P04" {
			return nil
		}
		return err
	}
	_, err = c.db.Exec(fmt.Sprintf(GRANT_PRIVS, dbname, username))
	if err != nil {
		return err
	}
	return nil
}

func (c *pg) CreateRole(role, password string) error {
	_, err := c.db.Exec(fmt.Sprintf(CREATE_ROLE, role, password))
	if err != nil {
		return err
	}
	return nil
}

func (c *pg) DropRole(role, database string, logger logr.Logger) error {
	// REASSIGN OWNED BY only works if the correct database is selected
	tmpDb := GetConnection(c.user, c.pass, c.host, database, c.args, logger)
	_, err := tmpDb.Exec(fmt.Sprintf(REASIGN_OBJECTS, role, "postgres"))
	defer tmpDb.Close()
	if err != nil && err.(*pq.Error).Code != "42704" {
		return err
	}

	_, err = c.db.Exec(fmt.Sprintf(DROP_PRIVS, database, role))
	if err != nil && err.(*pq.Error).Code != "3D000" {
		return err
	}

	_, err = c.db.Exec(fmt.Sprintf(DROP_ROLE, role))
	if err != nil && err.(*pq.Error).Code != "42704" {
		return err
	}
	return nil
}

func (c *pg) UpdatePassword(role, password string) error {
	_, err := c.db.Exec(fmt.Sprintf(UPDATE_PASSWORD, role, password))
	if err != nil {
		return err
	}

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
