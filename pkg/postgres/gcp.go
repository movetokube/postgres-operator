package postgres

import (
	"fmt"

	"github.com/go-logr/logr"
	"github.com/lib/pq"
)

type gcppg struct {
	pg
}

func newGCPPG(postgres *pg) PG {
	return &gcppg{
		*postgres,
	}
}

func (c *gcppg) CreateDB(dbname, role string) error {

	err := c.GrantRole(role, c.user)
	if err != nil {
		return err
	}
	err = c.pg.CreateDB(dbname, role)
	if err != nil {
		return err
	}
	return nil
}

func (c *gcppg) DropRole(role, newOwner, database string, logger logr.Logger) error {
	if role == "alloydbsuperuser" || role == "postgres" {
		logger.Info(fmt.Sprintf("not dropping %s as it is a reserved AlloyDB role", role))
		return nil
	}
	// On AlloyDB the postgres or other alloydbsuperuser members, aren't really superusers so they don't have permissions
	// to REASSIGN OWNED BY unless he belongs to both roles
	err := c.GrantRole(role, c.user)
	if err != nil && err.(*pq.Error).Code != "0LP01" {
		if err.(*pq.Error).Code == "42704" {
			// The group role does not exist, no point in continuing
			return nil
		}
		return err
	}
	defer c.RevokeRole(role, c.user)
	if newOwner != c.user {
		err = c.GrantRole(newOwner, c.user)
		if err != nil && err.(*pq.Error).Code != "0LP01" {
			if err.(*pq.Error).Code == "42704" {
				// The group role does not exist, no point of granting roles
				logger.Info(fmt.Sprintf("not granting %s to %s as %s does not exist", role, newOwner, newOwner))
				return nil
			}
			return err
		}
		defer c.RevokeRole(newOwner, c.user)
	}

	return c.pg.DropRole(role, newOwner, database, logger)
}
