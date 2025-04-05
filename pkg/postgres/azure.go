package postgres

import (
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	"github.com/lib/pq"
)

type AzureType string

const (
	// Azure Database for PostgreSQL Flexible Server uses default convention for login, but has not full superuser privileges
	FLEXIBLE AzureType = "flexible"
	// Azure Database for PostgreSQL Single Server uses <username>@<servername> convention
	SINGLE AzureType = "single"
)

type azurepg struct {
	serverName string
	azureType  AzureType
	pg
}

func newAzurePG(postgres *pg) PG {
	splitUser := strings.Split(postgres.user, "@")
	serverName := ""
	azureType := FLEXIBLE
	if len(splitUser) > 1 {
		// If a servername is found, we are using Azure Database for PostgreSQL Single Server
		serverName = splitUser[1]
		azureType = SINGLE
	}
	return &azurepg{
		serverName: serverName,
		azureType:  azureType,
		pg:         *postgres,
	}
}

func (azpg *azurepg) CreateUserRole(role, password string) (string, error) {
	returnedRole, err := azpg.pg.CreateUserRole(role, password)
	if err != nil {
		return "", err
	}

	// For Flexible Server, just return the role name as-is
	if azpg.azureType == FLEXIBLE {
		return returnedRole, nil
	}

	// For Single Server, format as <username>@<servername>
	return fmt.Sprintf("%s@%s", returnedRole, azpg.serverName), nil
}

func (azpg *azurepg) GetRoleForLogin(login string) string {
	// For Azure Flexible Server, the login name is the same as the role name
	if azpg.azureType == FLEXIBLE {
		return login
	}

	// For Azure Single Server, extract the username part before the '@' symbol
	splitUser := strings.Split(azpg.user, "@")
	return splitUser[0]
}

func (azpg *azurepg) CreateDB(dbname, role string) error {
	// This step is necessary before we can set the specified role as the database owner
	err := azpg.GrantRole(role, azpg.GetRoleForLogin(azpg.user))
	if err != nil {
		return err
	}

	return azpg.pg.CreateDB(dbname, role)
}

func (azpg *azurepg) DropRole(role, newOwner, database string, logger logr.Logger) error {
	if azpg.azureType == FLEXIBLE {
		// Grant the role to the user first
		err := azpg.GrantRole(role, azpg.user)
		if err != nil && err.(*pq.Error).Code != "0LP01" {
			if err.(*pq.Error).Code == "42704" {
				return nil
			}
			return err
		}

		// Delegate to parent implementation to perform the actual drop
		return azpg.pg.DropRole(role, newOwner, database, logger)
	}

	// For Azure Single Server, format the new owner correctly
	azNewOwner := azpg.GetRoleForLogin(newOwner)
	return azpg.pg.DropRole(role, azNewOwner, database, logger)
}
