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
	DropRole(role, database string) error
}

type pg struct {
	db  *sql.DB
	url string
}

func NewPG(url string, logger logr.Logger) (*pg, error) {
	db, err := sql.Open("postgres", url)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}
	logger.Info("connected to postgres server")
	return &pg{
		db:  db,
		url: url,
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

func (c *pg) DropRole(role, database string) error {
	_, err := c.db.Exec(fmt.Sprintf(REASIGN_OBJECTS, role, "postgres"))
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
