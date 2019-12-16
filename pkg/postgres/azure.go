package postgres

import (
	"fmt"
	"strings"
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

func (azpg *azurepg) Connect() error {
	// Default database for azure is postgres
	// https://docs.microsoft.com/en-us/azure/postgresql/concepts-servers#managing-your-server
	azpg.db = GetConnection(azpg.user, azpg.pass, azpg.host, "postgres", azpg.args, azpg.log)
	return nil
}
