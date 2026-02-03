package postgres

import (
	"fmt"
)

const (
	CREATE_DB               = `CREATE DATABASE "%s"`
	CREATE_SCHEMA           = `CREATE SCHEMA IF NOT EXISTS "%s" AUTHORIZATION "%s"`
	CREATE_EXTENSION        = `CREATE EXTENSION IF NOT EXISTS "%s"`
	ALTER_DB_OWNER          = `ALTER DATABASE "%s" OWNER TO "%s"`
	REASSIGN_DB_OWNER       = `REASSIGN OWNED BY "%s" TO "%s"`
	DROP_DATABASE           = `DROP DATABASE "%s" WITH (FORCE)`
	GRANT_USAGE_SCHEMA      = `GRANT USAGE ON SCHEMA "%s" TO "%s"`
	GRANT_CREATE_TABLE      = `GRANT CREATE ON SCHEMA "%s" TO "%s"`
	GRANT_ALL_TABLES        = `GRANT %s ON ALL TABLES IN SCHEMA "%s" TO "%s"`
	DEFAULT_PRIVS_SCHEMA    = `ALTER DEFAULT PRIVILEGES IN SCHEMA "%s" GRANT %s ON TABLES TO "%s"`
	GRANT_ALL_FUNCTIONS     = `GRANT %s ON ALL FUNCTIONS IN SCHEMA "%s" TO "%s"`
	DEFAULT_PRIVS_FUNCTIONS = `ALTER DEFAULT PRIVILEGES IN SCHEMA "%s" GRANT %s ON FUNCTIONS TO "%s"`
	GRANT_ALL_SEQUENCES     = `GRANT %s ON ALL SEQUENCES IN SCHEMA "%s" TO "%s"`
	DEFAULT_PRIVS_SEQUENCES = `ALTER DEFAULT PRIVILEGES IN SCHEMA "%s" GRANT %s ON SEQUENCES TO "%s"`
	REVOKE_CONNECT          = `REVOKE CONNECT ON DATABASE "%s" FROM public`
	GET_DB_OWNER            = `SELECT pg_catalog.pg_get_userbyid(d.datdba) FROM pg_catalog.pg_database d WHERE d.datname = '%s'`
	GRANT_CREATE_SCHEMA     = `GRANT CREATE ON DATABASE "%s" TO "%s"`
	GRANT_CONNECT           = `GRANT CONNECT ON DATABASE "%s" TO "%s"`
)

func (c *pg) CreateDB(dbname, role string) error {
	// Create database
	err := c.execute(fmt.Sprintf(CREATE_DB, dbname))
	if err != nil {
		// eat DUPLICATE DATABASE ERROR
		if !isPgError(err, "42P04") {
			return err
		}
	}

	err = c.execute(fmt.Sprintf(ALTER_DB_OWNER, dbname, role))
	if err != nil {
		return err
	}

	// Grant CREATE on database to owner and operator user
	usersToGrant := []string{c.user, role}
	for _, u := range usersToGrant {
		err = c.execute(fmt.Sprintf(GRANT_CREATE_SCHEMA, dbname, u))
		if err != nil {
			return fmt.Errorf("failed to grant create schema on %s to %s: %w", dbname, u, err)
		}
	}
	// Grant CONNECT on database to owner and operator user
	for _, u := range usersToGrant {
		err = c.execute(fmt.Sprintf(GRANT_CONNECT, dbname, u))
		if err != nil {
			return fmt.Errorf("failed to grant connect on %s to %s: %w", dbname, u, err)
		}
	}
	return nil
}

// reconcile the desired owner of the database
func (c *pg) AlterDatabaseOwner(dbname, owner string) error {
	if owner == "" {
		return nil
	}
	return c.execute(fmt.Sprintf(ALTER_DB_OWNER, dbname, owner))
}

func (c *pg) ReassignDatabaseOwner(dbName, currentOwner, newOwner string) error {
	if currentOwner == "" || newOwner == "" || currentOwner == newOwner {
		return nil
	}

	tmpDb, err := getConnection(c.user, c.pass, c.host, dbName, c.args)
	if err != nil {
		return err
	}
	defer tmpDb.Close()

	_, err = tmpDb.Exec(fmt.Sprintf(REASSIGN_DB_OWNER, currentOwner, newOwner))
	if err != nil {
		if isPgError(err, "42704") {
			return nil
		}
		return err
	}
	return nil
}

func (c *pg) CreateSchema(db, role, schema string) error {
	tmpDb, err := getConnection(c.user, c.pass, c.host, db, c.args)
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

func (c *pg) DropDatabase(database string) error {
	err := c.execute(fmt.Sprintf(REVOKE_CONNECT, database))
	// Error code 3D000 is returned if database doesn't exist
	if err != nil && !isPgError(err, "3D000") {
		return err
	}

	err = c.execute(fmt.Sprintf(DROP_DATABASE, database))
	// Error code 3D000 is returned if database doesn't exist
	if err != nil && !isPgError(err, "3D000") {
		return err
	}

	c.log.Info(fmt.Sprintf("Dropped database %s", database))

	return nil
}

func (c *pg) CreateExtension(db, extension string) error {
	tmpDb, err := getConnection(c.user, c.pass, c.host, db, c.args)
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

func (c *pg) SetSchemaPrivileges(schemaPrivileges PostgresSchemaPrivileges) error {
	tmpDb, err := getConnection(c.user, c.pass, c.host, schemaPrivileges.DB, c.args)
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
