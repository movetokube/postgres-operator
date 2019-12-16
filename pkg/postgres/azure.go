package postgres

import (
	"fmt"
	"strings"

	"github.com/lib/pq"
)

type azurepg struct {
	pg
}

func newAzurePG(postgres *pg) PG {
	return &azurepg{
		*postgres,
	}
}

func (azpg *azurepg) GetLoginForRole(role string) string {
	splitUser := strings.Split(azpg.user, "@")
	if len(splitUser) > 1 {
		return fmt.Sprintf("%s@%s", role, splitUser[1])
	}
	// Fallback to the role name if there was no <@server> added in the login user, but mostly this will mean there is some wrong configuration
	return role
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
