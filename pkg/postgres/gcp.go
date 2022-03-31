package postgres

/*
To have operator work with GCP you have 
1) use postgresql connection in secret
2) manually create a Master role e.g. "devops-operators"
3) use such role in database CR e.g. spec.masterRole: devops-operator

DropRole method will check for db owner and will skip master role dropping
*/
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

	_, err := c.db.Exec(fmt.Sprintf("REVOKE CONNECT ON DATABASE \"%s\" FROM public;", database))
	// Error code 3D000 is returned if database doesn't exist
	if err != nil && err.(*pq.Error).Code != "3D000" {
		return err
	}

	_, err = c.db.Exec(fmt.Sprintf("select pg_terminate_backend(pg_stat_activity.pid) FROM pg_stat_activity	WHERE pg_stat_activity.datname = '%s' AND pid <> pg_backend_pid();", database))
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
	q := fmt.Sprintf("SELECT pg_catalog.pg_get_userbyid(d.datdba) FROM pg_catalog.pg_database d WHERE d.datname = '%s';", database)
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
