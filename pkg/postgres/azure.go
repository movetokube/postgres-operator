package postgres

import (
	"fmt"
	"strings"

	"github.com/go-logr/logr"
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
	returnedRole, err := azpg.pg.CreateUserRole(role, password)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s@%s", returnedRole, azpg.serverName), nil
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

func (azpg *azurepg) DropRole(role, newOwner, database string, logger logr.Logger) error {
	// REASSIGN OWNED BY only works if the correct database is selected
	tmpDb := GetConnection(azpg.user, azpg.pass, azpg.host, database, azpg.args, logger)
	_, err := tmpDb.Exec(fmt.Sprintf(REASIGN_OBJECTS, role, azpg.GetRoleForLogin(newOwner)))
	defer tmpDb.Close()
	// Check if error exists and if different from "ROLE NOT FOUND" => 42704
	if err != nil && err.(*pq.Error).Code != "42704" {
		return err
	}

	// We previously assigned all objects to the operator's role so DROP OWNED BY will drop privileges of role
	_, err = tmpDb.Exec(fmt.Sprintf(DROP_OWNED_BY, role))
	// Check if error exists and if different from "ROLE NOT FOUND" => 42704
	if err != nil && err.(*pq.Error).Code != "42704" {
		return err
	}

	_, err = azpg.db.Exec(fmt.Sprintf(DROP_ROLE, role))
	// Check if error exists and if different from "ROLE NOT FOUND" => 42704
	if err != nil && err.(*pq.Error).Code != "42704" {
		return err
	}
	return nil
}
