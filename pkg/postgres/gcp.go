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

func (c *gcppg) DropDatabase(database string, logger logr.Logger) error {

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
	
	tmpDb, err := GetConnection(c.user, c.pass, c.host, database, c.args, logger)
	q := fmt.Sprintf(GET_DB_OWNER, database)
	logger.Info("Checking master role: "+ q)
	rows, err := tmpDb.Query(q)
	if err != nil {
		return err
	}
	var masterRole string
	for rows.Next() {
		rows.Scan(&masterRole)
	}

	if( role != masterRole){
		q = fmt.Sprintf(DROP_ROLE, role)
		logger.Info("GCP Drop Role: "+ q)
		_, err = tmpDb.Exec(q)
		// Check if error exists and if different from "ROLE NOT FOUND" => 42704
		if err != nil && err.(*pq.Error).Code != "42704" {
			return err
		}

		defer tmpDb.Close()
	} else {
		logger.Info(fmt.Sprintf("GCP refusing DropRole on master role " + masterRole))
	}
	return nil
}
