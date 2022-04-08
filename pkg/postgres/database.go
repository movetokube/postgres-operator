package postgres

import (
	"fmt"

	"github.com/go-logr/logr"
	"github.com/lib/pq"
)

const (
	CREATE_DB            = `CREATE DATABASE "%s"`
	CREATE_SCHEMA        = `CREATE SCHEMA IF NOT EXISTS "%s" AUTHORIZATION "%s"`
	CREATE_EXTENSION     = `CREATE EXTENSION IF NOT EXISTS "%s"`
	ALTER_DB_OWNER       = `ALTER DATABASE "%s" OWNER TO "%s"`
	DROP_DATABASE        = `DROP DATABASE "%s"`
	GRANT_USAGE_SCHEMA   = `GRANT USAGE ON SCHEMA "%s" TO "%s"`
	GRANT_ALL_TABLES     = `GRANT %s ON ALL TABLES IN SCHEMA "%s" TO "%s"`
	DEFAULT_PRIVS_SCHEMA = `ALTER DEFAULT PRIVILEGES FOR ROLE "%s" IN SCHEMA "%s" GRANT %s ON TABLES TO "%s"`
	REVOKE_CONNECT		 = `REVOKE CONNECT ON DATABASE "%s" FROM public`
	TERMINATE_BACKEND	 = `SELECT pg_terminate_backend(pg_stat_activity.pid) FROM pg_stat_activity	WHERE pg_stat_activity.datname = '%s' AND pid <> pg_backend_pid()`
	GET_DB_OWNER	 	 = `SELECT pg_catalog.pg_get_userbyid(d.datdba) FROM pg_catalog.pg_database d WHERE d.datname = '%s'`
	GRANT_CREATE_SCHEMA  = `GRANT CREATE ON DATABASE "%s" TO "%s"`
)

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

	_, err = c.db.Exec(fmt.Sprintf(GRANT_CREATE_SCHEMA, dbname, role))
	if err != nil {
		return err
	}
	return nil
}

func (c *pg) CreateSchema(db, role, schema string, logger logr.Logger) error {
	tmpDb, err := GetConnection(c.user, c.pass, c.host, db, c.args, logger)
	if err != nil {
		return err
	}
	defer tmpDb.Close()

	_, err = tmpDb.Exec(fmt.Sprintf(CREATE_SCHEMA, schema, role))
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

func (c *pg) CreateExtension(db, extension string, logger logr.Logger) error {
	tmpDb, err := GetConnection(c.user, c.pass, c.host, db, c.args, logger)
	if err != nil {
		return err
	}
	defer tmpDb.Close()

	_, err = tmpDb.Exec(fmt.Sprintf(CREATE_EXTENSION, extension))
	if err != nil {
		return err
	}
	return nil
}

func (c *pg) SetSchemaPrivileges(db, creator, role, schema, privs string, logger logr.Logger) error {
	tmpDb, err := GetConnection(c.user, c.pass, c.host, db, c.args, logger)
	if err != nil {
		return err
	}
	defer tmpDb.Close()

	// Grant role usage on schema
	_, err = tmpDb.Exec(fmt.Sprintf(GRANT_USAGE_SCHEMA, schema, role))
	if err != nil {
		return err
	}

	// Grant role privs on existing tables in schema
	_, err = tmpDb.Exec(fmt.Sprintf(GRANT_ALL_TABLES, privs, schema, role))
	if err != nil {
		return err
	}

	// Grant role privs on future tables in schema
	_, err = tmpDb.Exec(fmt.Sprintf(DEFAULT_PRIVS_SCHEMA, creator, schema, privs, role))
	if err != nil {
		return err
	}
	return nil
}
