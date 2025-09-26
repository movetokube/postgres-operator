package postgres

import (
	"github.com/lib/pq"
)

type AzureType string

type azurepg struct {
	pg
}

func newAzurePG(postgres *pg) PG {
	return &azurepg{
		pg: *postgres,
	}
}

func (azpg *azurepg) CreateUserRole(role, password string) (string, error) {
	returnedRole, err := azpg.pg.CreateUserRole(role, password)
	if err != nil {
		return "", err
	}
	return returnedRole, nil
}

func (azpg *azurepg) CreateDB(dbname, role string) error {
	// This step is necessary before we can set the specified role as the database owner
	err := azpg.GrantRole(role, azpg.user)
	if err != nil {
		return err
	}

	return azpg.pg.CreateDB(dbname, role)
}

func (azpg *azurepg) DropRole(role, newOwner, database string) error {
	// Grant the role to the user first
	err := azpg.GrantRole(role, azpg.user)
	if err != nil && err.(*pq.Error).Code != "0LP01" {
		if err.(*pq.Error).Code == "42704" {
			return nil
		}
		return err
	}

	// Delegate to parent implementation to perform the actual drop
	return azpg.pg.DropRole(role, newOwner, database)
}
