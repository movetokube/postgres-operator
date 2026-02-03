package postgres

import (
	"fmt"
)

const (
	CREATE_GROUP_ROLE   = `CREATE ROLE "%s"`
	RENAME_GROUP_ROLE   = `ALTER ROLE "%s" RENAME TO "%s"`
	CREATE_USER_ROLE    = `CREATE ROLE "%s" WITH LOGIN PASSWORD '%s'`
	GRANT_ROLE          = `GRANT "%s" TO "%s"`
	ALTER_USER_SET_ROLE = `ALTER USER "%s" SET ROLE "%s"`
	REVOKE_ROLE         = `REVOKE "%s" FROM "%s"`
	UPDATE_PASSWORD     = `ALTER ROLE "%s" WITH PASSWORD '%s'`
	DROP_ROLE           = `DROP ROLE "%s"`
	DROP_OWNED_BY       = `DROP OWNED BY "%s"`
	REASIGN_OBJECTS     = `REASSIGN OWNED BY "%s" TO "%s"`
)

var reservedRoles = map[string]struct{}{
	"alloydbsuperuser":  {}, // GCP AlloyDB
	"cloudsqlsuperuser": {}, // GCP Cloud SQL
	"rdsadmin":          {}, // AWS RDS
	"azuresu":           {}, // Azure Database for PostgreSQL
}

func (c *pg) CreateGroupRole(role string) error {
	// Error code 42710 is duplicate_object (role already exists)
	err := c.execute(fmt.Sprintf(CREATE_GROUP_ROLE, role))
	if err != nil && !isPgError(err, "42710") {
		return err
	}
	// Grant role also to the operator role to be able to manage it
	err = c.GrantRole(role, c.user)
	if err != nil && !isPgError(err, "0LP01") {
		return err
	}

	return nil
}

func (c *pg) RenameGroupRole(currentRole, newRole string) error {
	err := c.execute(fmt.Sprintf(RENAME_GROUP_ROLE, currentRole, newRole))
	if err != nil {
		// 42704 => role does not exist; treat as success so caller can recreate
		if isPgError(err, "42704") {
			return nil
		}
		return err
	}
	return nil
}

func (c *pg) CreateUserRole(role, password string) (string, error) {
	err := c.execute(fmt.Sprintf(CREATE_USER_ROLE, role, password))
	if err != nil {
		return "", err
	}

	err = c.GrantRole(role, c.user)
	if err != nil && !isPgError(err, "0LP01") {
		return "", err
	}
	return role, nil
}

func (c *pg) GrantRole(role, grantee string) error {
	return c.execute(fmt.Sprintf(GRANT_ROLE, role, grantee))
}

func (c *pg) AlterDefaultLoginRole(role, setRole string) error {
	return c.execute(fmt.Sprintf(ALTER_USER_SET_ROLE, role, setRole))
}

func (c *pg) RevokeRole(role, revoked string) error {
	return c.execute(fmt.Sprintf(REVOKE_ROLE, role, revoked))
}

func (c *pg) DropRole(role, newOwner, database string) error {
	if _, reserved := reservedRoles[role]; reserved || role == c.user {
		c.log.Info(fmt.Sprintf("not dropping %s as it is a reserved role", role))
		return nil
	}

	err := c.GrantRole(role, c.user)
	if err != nil && !isPgError(err, "0LP01") {
		if isPgError(err, "42704") {
			// The group role does not exist, no point in continuing
			return nil
		}
		return err
	}
	defer func() {
		if err := c.RevokeRole(role, c.user); err != nil && !isPgError(err, "0LP01") {
			c.log.Error(err, "failed to revoke role from operator", "role", role)
		}
	}()
	if newOwner != c.user {
		err = c.GrantRole(newOwner, c.user)
		if err != nil && !isPgError(err, "0LP01") {
			if isPgError(err, "42704") {
				// The group role does not exist, no point of granting roles
				c.log.Info(fmt.Sprintf("not granting %s to %s as %s does not exist", role, newOwner, newOwner))
				return nil
			}
			return err
		}
		defer func() {
			if err := c.RevokeRole(newOwner, c.user); err != nil && !isPgError(err, "0LP01") {
				c.log.Error(err, "failed to revoke newOwner role from operator", "role", newOwner)
			}
		}()
	}

	// REASSIGN OWNED BY only works if the correct database is selected
	tmpDb, err := getConnection(c.user, c.pass, c.host, database, c.args)
	if err != nil {
		if isPgError(err, "3D000") {
			return nil // Database is does not exist (anymore)
		} else {
			return err
		}
	}
	_, err = tmpDb.Exec(fmt.Sprintf(REASIGN_OBJECTS, role, newOwner))
	defer tmpDb.Close()
	// Check if error exists and if different from "ROLE NOT FOUND" => 42704
	if err != nil && !isPgError(err, "42704") {
		return err
	}

	// We previously assigned all objects to the operator's role so DROP OWNED BY will drop privileges of role
	_, err = tmpDb.Exec(fmt.Sprintf(DROP_OWNED_BY, role))
	// Check if error exists and if different from "ROLE NOT FOUND" => 42704
	if err != nil && !isPgError(err, "42704") {
		return err
	}

	err = c.execute(fmt.Sprintf(DROP_ROLE, role))
	// Check if error exists and if different from "ROLE NOT FOUND" => 42704
	if err != nil && !isPgError(err, "42704") {
		return err
	}
	return nil
}

func (c *pg) UpdatePassword(role, password string) error {
	return c.execute(fmt.Sprintf(UPDATE_PASSWORD, role, password))
}
