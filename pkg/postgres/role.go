package postgres

import (
	"fmt"

	"github.com/go-logr/logr"
	"github.com/lib/pq"
)

const (
	CREATE_GROUP_ROLE   = `CREATE ROLE "%s"`
	CREATE_USER_ROLE    = `CREATE ROLE "%s" WITH LOGIN PASSWORD '%s'`
	GRANT_ROLE          = `GRANT "%s" TO "%s"`
	ALTER_USER_SET_ROLE = `ALTER USER "%s" SET ROLE "%s"`
	REVOKE_ROLE         = `REVOKE "%s" FROM "%s"`
	UPDATE_PASSWORD     = `ALTER ROLE "%s" WITH PASSWORD '%s'`
	DROP_ROLE           = `DROP ROLE "%s"`
	DROP_OWNED_BY       = `DROP OWNED BY "%s"`
	REASIGN_OBJECTS     = `REASSIGN OWNED BY "%s" TO "%s"`
)

func (c *pg) CreateGroupRole(role string) error {
	// Error code 42710 is duplicate_object (role already exists)
	_, err := c.db.Exec(fmt.Sprintf(CREATE_GROUP_ROLE, role))
	if err != nil && err.(*pq.Error).Code != "42710" {
		return err
	}
	return nil
}

func (c *pg) CreateUserRole(role, password string) (string, error) {
	_, err := c.db.Exec(fmt.Sprintf(CREATE_USER_ROLE, role, password))
	if err != nil {
		return "", err
	}
	return role, nil
}

func (c *pg) GrantRole(role, grantee string) error {
	_, err := c.db.Exec(fmt.Sprintf(GRANT_ROLE, role, grantee))
	if err != nil {
		return err
	}
	return nil
}

func (c *pg) AlterDefaultLoginRole(role, setRole string) error {
	_, err := c.db.Exec(fmt.Sprintf(ALTER_USER_SET_ROLE, role, setRole))
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

func (c *pg) DropRole(role, newOwner, database string, logger logr.Logger) error {
	// REASSIGN OWNED BY only works if the correct database is selected
	tmpDb, err := GetConnection(c.user, c.pass, c.host, database, c.args, logger)
	if err != nil {
		if err.(*pq.Error).Code == "3D000" {
			return nil // Database is does not exist (anymore)
		} else {
			return err
		}
	}
	_, err = tmpDb.Exec(fmt.Sprintf(REASIGN_OBJECTS, role, newOwner))
	defer tmpDb.Close()
	// Check if error exists and if different from "ROLE NOT FOUND" => 42704
	if err != nil && err.(*pq.Error).Code != "42704" {
		return err
	}

	// We previously assigned all objects to the operator's role so DROP OWNED BY will drop privileges of role
	_, err = tmpDb.Exec(fmt.Sprintf(DROP_OWNED_BY, role))
	// Check if error exists and if different from "ROLE NOT FOUND" => 42704
	if err != nil && err.(*pq.Error).Code != "42704" {
		return err
	}

	_, err = c.db.Exec(fmt.Sprintf(DROP_ROLE, role))
	// Check if error exists and if different from "ROLE NOT FOUND" => 42704
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
