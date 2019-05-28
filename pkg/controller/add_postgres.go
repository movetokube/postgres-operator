package controller

import (
	"github.com/hitman99/postgres-operator/pkg/controller/postgres"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, postgres.Add)
}
