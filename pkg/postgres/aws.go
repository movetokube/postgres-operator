package postgres

import (
	"fmt"

	"github.com/go-logr/logr"
	"github.com/lib/pq"
)

type awspg struct {
	pg
}

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

func (c *awspg) DropRole(role, newOwner, database string, logger logr.Logger) error {
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
			logger.Info(fmt.Sprintf("not granting %s to %s as %s does not exist", role, newOwner, newOwner))

			_, err = c.pg.db.Exec(fmt.Sprintf(DROP_ROLE, role))
			if err != nil && err.(*pq.Error).Code != "42704" {
				return err
			}
			logger.Info(fmt.Sprintf("role \"%s\" has been dropped", role))
			return nil
		}
		return err
	}
	defer c.RevokeRole(newOwner, c.user)

	return c.pg.DropRole(role, newOwner, database, logger)
}
