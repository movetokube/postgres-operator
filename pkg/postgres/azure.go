package postgres

import (
	"fmt"
	"strings"
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
	_, err := azpg.pg.CreateUserRole(role, password)
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

func (azpg *azurepg) CreateDB(dbname, role string) error {
	// Have to add the master role to the group role before we can transfer the database owner
	err := azpg.GrantRole(role, azpg.GetRoleForLogin(azpg.user))
	if err != nil {
		return err
	}

	return azpg.pg.CreateDB(dbname, role)
}
