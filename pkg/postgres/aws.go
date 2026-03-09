package postgres

import (
	"fmt"
	"strings"

	"github.com/lib/pq"
)

type awspg struct {
	pg
}

const (
	AWS_ALTER_REPACK_DEFAULT_PRIVS_TABLES    = `ALTER DEFAULT PRIVILEGES FOR ROLE "%s" IN SCHEMA "repack" GRANT INSERT ON TABLES TO PUBLIC`
	AWS_ALTER_REPACK_DEFAULT_PRIVS_SEQUENCES = `ALTER DEFAULT PRIVILEGES FOR ROLE "%s" IN SCHEMA "repack" GRANT USAGE, SELECT ON SEQUENCES TO PUBLIC`
)

func newAWSPG(postgres *pg) PG {
	return &awspg{
		*postgres,
	}
}

func (c *awspg) AlterDefaultLoginRole(role, setRole string) error {
	// On AWS RDS the postgres user isn't really superuser so he doesn't have permissions
	// to ALTER USER unless he belongs to both roles
	err := c.GrantRole(role, c.user)
	if err != nil {
		return err
	}
	defer c.RevokeRole(role, c.user)

	return c.pg.AlterDefaultLoginRole(role, setRole)
}

func (c *awspg) CreateDB(dbname, role string) error {
	// Have to add the master role to the group role before we can transfer the database owner
	err := c.GrantRole(role, c.user)
	if err != nil {
		return err
	}

	return c.pg.CreateDB(dbname, role)
}

func (c *awspg) CreateExtension(dbname, extension string) error {
	// Keep standard extension creation behavior for AWS as well.
	err := c.pg.CreateExtension(dbname, extension)
	if err != nil {
		return err
	}

	// AWS-specific workaround is only required for pg_repack.
	if !strings.EqualFold(extension, "pg_repack") {
		return nil
	}

	var owner string
	// Resolve current database owner role to target ALTER DEFAULT PRIVILEGES FOR ROLE.
	err = c.db.QueryRow(fmt.Sprintf(GET_DB_OWNER, dbname)).Scan(&owner)
	if err != nil {
		return err
	}

	// Execute pg_repack privilege statements in the target database.
	tmpDb, err := GetConnection(c.user, c.pass, c.host, dbname, c.args)
	if err != nil {
		return err
	}
	defer tmpDb.Close()

	_, err = tmpDb.Exec(fmt.Sprintf(AWS_ALTER_REPACK_DEFAULT_PRIVS_TABLES, owner))
	if err != nil {
		return err
	}

	_, err = tmpDb.Exec(fmt.Sprintf(AWS_ALTER_REPACK_DEFAULT_PRIVS_SEQUENCES, owner))
	if err != nil {
		return err
	}

	return nil
}

func (c *awspg) CreateUserRole(role, password string) (string, error) {
	returnedRole, err := c.pg.CreateUserRole(role, password)
	if err != nil {
		return "", err
	}
	// On AWS RDS the postgres user isn't really superuser so he doesn't have permissions
	// to ALTER DEFAULT PRIVILEGES FOR ROLE unless he belongs to the role
	err = c.GrantRole(role, c.user)
	if err != nil {
		return "", err
	}

	return returnedRole, nil
}

func (c *awspg) DropRole(role, newOwner, database string) error {
	// On AWS RDS the postgres user isn't really superuser so he doesn't have permissions
	// to REASSIGN OWNED BY unless he belongs to both roles
	err := c.GrantRole(role, c.user)
	if err != nil && err.(*pq.Error).Code != "0LP01" {
		if err.(*pq.Error).Code == "42704" {
			// The group role does not exist, no point in continuing
			return nil
		}
		return err
	}
	err = c.GrantRole(newOwner, c.user)
	if err != nil && err.(*pq.Error).Code != "0LP01" {
		if err.(*pq.Error).Code == "42704" {
			// The group role does not exist, no point of granting roles
			c.log.Info(fmt.Sprintf("not granting %s to %s as %s does not exist", role, newOwner, newOwner))
			return nil
		}
		return err
	}
	defer c.RevokeRole(newOwner, c.user)

	return c.pg.DropRole(role, newOwner, database)
}
