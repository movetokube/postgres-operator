package postgres

import (
	"fmt"

	"github.com/go-logr/logr"
	"github.com/lib/pq"
)

const (
	CREATE_DB               = `CREATE DATABASE "%s"`
	CREATE_SCHEMA           = `CREATE SCHEMA IF NOT EXISTS "%s" AUTHORIZATION "%s"`
	CREATE_EXTENSION        = `CREATE EXTENSION IF NOT EXISTS "%s"`
	ALTER_DB_OWNER          = `ALTER DATABASE "%s" OWNER TO "%s"`
	DROP_DATABASE           = `DROP DATABASE "%s"`
	GRANT_USAGE_SCHEMA      = `GRANT USAGE ON SCHEMA "%s" TO "%s"`
	GRANT_CREATE_TABLE      = `GRANT CREATE ON SCHEMA "%s" TO "%s"`
	GRANT_ALL_TABLES        = `GRANT %s ON ALL TABLES IN SCHEMA "%s" TO "%s"`
	DEFAULT_PRIVS_SCHEMA    = `ALTER DEFAULT PRIVILEGES IN SCHEMA "%s" GRANT %s ON TABLES TO "%s"`
	GRANT_ALL_FUNCTIONS     = `GRANT %s ON ALL FUNCTIONS IN SCHEMA "%s" TO "%s"`
	DEFAULT_PRIVS_FUNCTIONS = `ALTER DEFAULT PRIVILEGES IN SCHEMA "%s" GRANT %s ON FUNCTIONS TO "%s"`
	GRANT_ALL_SEQUENCES     = `GRANT %s ON ALL SEQUENCES IN SCHEMA "%s" TO "%s"`
	DEFAULT_PRIVS_SEQUENCES = `ALTER DEFAULT PRIVILEGES IN SCHEMA "%s" GRANT %s ON SEQUENCES TO "%s"`
	REVOKE_CONNECT          = `REVOKE CONNECT ON DATABASE "%s" FROM public`
	TERMINATE_BACKEND       = `SELECT pg_terminate_backend(pg_stat_activity.pid) FROM pg_stat_activity	WHERE pg_stat_activity.datname = '%s' AND pid <> pg_backend_pid()`
	GET_DB_OWNER            = `SELECT pg_catalog.pg_get_userbyid(d.datdba) FROM pg_catalog.pg_database d WHERE d.datname = '%s'`
	GRANT_CREATE_SCHEMA     = `GRANT CREATE ON DATABASE "%s" TO "%s"`
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

// reconcile the desired owner of the database
func (c *pg) AlterDatabaseOwner(dbname, owner string) error {
	_, err := c.db.Exec(fmt.Sprintf(ALTER_DB_OWNER, dbname, owner))
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
	_, err := c.db.Exec(fmt.Sprintf(REVOKE_CONNECT, database))
	// Error code 3D000 is returned if database doesn't exist
	if err != nil && err.(*pq.Error).Code != "3D000" {
		return err
	}

	_, err = c.db.Exec(fmt.Sprintf(TERMINATE_BACKEND, database))
	// Error code 3D000 is returned if database doesn't exist
	if err != nil && err.(*pq.Error).Code != "3D000" {
		return err
	}
	_, err = c.db.Exec(fmt.Sprintf(DROP_DATABASE, database))
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

func (c *pg) SetSchemaPrivileges(schemaPrivileges PostgresSchemaPrivileges, logger logr.Logger) error {
	tmpDb, err := GetConnection(c.user, c.pass, c.host, schemaPrivileges.DB, c.args, logger)
	if err != nil {
		return err
	}
	defer tmpDb.Close()

	// Grant role usage on schema
	_, err = tmpDb.Exec(fmt.Sprintf(GRANT_USAGE_SCHEMA, schemaPrivileges.Schema, schemaPrivileges.Role))
	if err != nil {
		return err
	}

	// Grant role privs on existing tables in schema
	_, err = tmpDb.Exec(fmt.Sprintf(GRANT_ALL_TABLES, schemaPrivileges.Privs, schemaPrivileges.Schema, schemaPrivileges.Role))
	if err != nil {
		return err
	}

	// Grant role privs on future tables in schema
	_, err = tmpDb.Exec(fmt.Sprintf(DEFAULT_PRIVS_SCHEMA, schemaPrivileges.Schema, schemaPrivileges.Privs, schemaPrivileges.Role))
	if err != nil {
		return err
	}

	if schemaPrivileges.SequencePrivs != "" {
		// Grant role privs on existing sequences in schema
		_, err = tmpDb.Exec(fmt.Sprintf(GRANT_ALL_SEQUENCES, schemaPrivileges.SequencePrivs, schemaPrivileges.Schema, schemaPrivileges.Role))
		if err != nil {
			return err
		}

		// Grant role privs on future sequences in schema
		_, err = tmpDb.Exec(fmt.Sprintf(DEFAULT_PRIVS_SEQUENCES, schemaPrivileges.Schema, schemaPrivileges.SequencePrivs, schemaPrivileges.Role))
		if err != nil {
			return err
		}
	}

	if schemaPrivileges.FunctionPrivs != "" {
		// Grant role privs on existing functions in schema
		_, err = tmpDb.Exec(fmt.Sprintf(GRANT_ALL_FUNCTIONS, schemaPrivileges.FunctionPrivs, schemaPrivileges.Schema, schemaPrivileges.Role))
		if err != nil {
			return err
		}

		// Grant role privs on future functions in schema
		_, err = tmpDb.Exec(fmt.Sprintf(DEFAULT_PRIVS_FUNCTIONS, schemaPrivileges.Schema, schemaPrivileges.FunctionPrivs, schemaPrivileges.Role))
		if err != nil {
			return err
		}
	}

	// Grant role usage on schema if createSchema
	if schemaPrivileges.CreateSchema {
		_, err = tmpDb.Exec(fmt.Sprintf(GRANT_CREATE_TABLE, schemaPrivileges.Schema, schemaPrivileges.Role))
		if err != nil {
			return err
		}
	}

	return nil
}
