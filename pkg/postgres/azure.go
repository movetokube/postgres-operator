package postgres

import (
	"fmt"
	"strings"

	"github.com/lib/pq"
)

type azurepg struct {
	serverName string
	pg
}

func newAzurePG(postgres *pg) PG {
	splitUser := strings.Split(postgres.user, "@")
	serverName := ""
	if len(splitUser) > 1 {
		serverName = splitUser[1]
	}
	return &azurepg{
		serverName,
		*postgres,
	}
}

func (azpg *azurepg) CreateUserRole(role, password string) (string, error) {
	_, err := azpg.db.Exec(fmt.Sprintf(CREATE_USER_ROLE, role, password))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s@%s", role, azpg.serverName), nil
}

func (azpg *azurepg) GetRoleForLogin(login string) string {
	splitUser := strings.Split(azpg.user, "@")
	if len(splitUser) > 1 {
		return splitUser[0]
	}
	return login
}

func (azpg *azurepg) Connect() error {
	// Default database for azure is postgres
	// https://docs.microsoft.com/en-us/azure/postgresql/concepts-servers#managing-your-server
	azpg.db = GetConnection(azpg.user, azpg.pass, azpg.host, "postgres", azpg.args, azpg.log)
	return nil
}

func (azpg *azurepg) CreateDB(dbname, role string) error {
	_, err := azpg.db.Exec(fmt.Sprintf(CREATE_DB, dbname))
	if err != nil {
		// eat DUPLICATE DATABASE ERROR
		if err.(*pq.Error).Code != "42P04" {
			return err
		}
	}
	// Have to add the master role to the group role before we can transfer the database owner
	err = azpg.GrantRole(role, azpg.GetRoleForLogin(azpg.user))
	if err != nil {
		return err
	}
	_, err = azpg.db.Exec(fmt.Sprintf(ALTER_DB_OWNER, dbname, role))
	if err != nil {
		return err
	}
	return nil
}
