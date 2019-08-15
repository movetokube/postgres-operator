package controller

import (
	"github.com/movetokube/postgres-operator/pkg/controller/postgresuser"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, postgresuser.Add)
}
