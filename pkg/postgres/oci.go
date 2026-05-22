package postgres

import (
	"fmt"

	"github.com/lib/pq"
)

type ocipg struct {
	pg
}

func newOCIPG(postgres *pg) PG {
	return &ocipg{
		*postgres,
	}
}

func (c *ocipg) CreateDB(dbname, role string) error {
	// OCI managed Postgres admin user is not a true superuser; must belong to
	// the role before ALTER DATABASE ... OWNER TO succeeds
	err := c.GrantRole(role, c.user)
	if err != nil {
		return err
	}

	return c.pg.CreateDB(dbname, role)
}

func (c *ocipg) DropRole(role, newOwner, database string) error {
	// Must belong to both roles for REASSIGN OWNED BY to succeed
	err := c.GrantRole(role, c.user)
	if err != nil && err.(*pq.Error).Code != "0LP01" {
		if err.(*pq.Error).Code == "42704" {
			return nil
		}
		return err
	}
	err = c.GrantRole(newOwner, c.user)
	if err != nil && err.(*pq.Error).Code != "0LP01" {
		if err.(*pq.Error).Code == "42704" {
			c.log.Info(fmt.Sprintf("not granting %s to %s as %s does not exist", role, newOwner, newOwner))
			return nil
		}
		return err
	}
	defer c.RevokeRole(newOwner, c.user)

	return c.pg.DropRole(role, newOwner, database)
}
