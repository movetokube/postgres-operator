package fake

import (
	"github.com/go-logr/logr"
)

func NewMockPostgres() *MockPostgres {
	return &MockPostgres{
		MockCreateDB: func(dbname, username string) error {
			return nil
		},
		MockCreateSchema: func(db, role, schema string, logger logr.Logger) error {
			return nil
		},
		MockCreateExtension: func(db, extension string, logger logr.Logger) error {
			return nil
		},
		MockCreateGroupRole: func(role string) error {
			return nil
		},
		MockCreateUserRole: func(role, password string) (string, error) {
			return "user", nil
		},
		MockUpdatePassword: func(role, password string) error {
			return nil
		},
		MockGrantRole: func(role, grantee string) error {
			return nil
		},
		MockSetSchemaPrivileges: func(db, creator, role, schema, privs string, logger logr.Logger) error {
			return nil
		},
		MockRevokeRole: func(role, revoked string) error {
			return nil
		},
		MockAlterDefaultLoginRole: func(role, setRole string) error {
			return nil
		},
		MockDropDatabase: func(db string, logger logr.Logger) error {
			return nil
		},
		MockDropRole: func(role, newOwner, database string, logger logr.Logger) error {
			return nil
		},
		MockGetUser: func() string {
			return "user"
		},
	}
}

type MockPostgres struct {
	MockCreateDB              func(dbname, username string) error
	MockCreateSchema          func(db, role, schema string, logger logr.Logger) error
	MockCreateExtension       func(db, extension string, logger logr.Logger) error
	MockCreateGroupRole       func(role string) error
	MockCreateUserRole        func(role, password string) (string, error)
	MockUpdatePassword        func(role, password string) error
	MockGrantRole             func(role, grantee string) error
	MockSetSchemaPrivileges   func(db, creator, role, schema, privs string, logger logr.Logger) error
	MockRevokeRole            func(role, revoked string) error
	MockAlterDefaultLoginRole func(role, setRole string) error
	MockDropDatabase          func(db string, logger logr.Logger) error
	MockDropRole              func(role, newOwner, database string, logger logr.Logger) error
	MockGetUser               func() string
}

func (p *MockPostgres) CreateDB(dbname, username string) error {
	return p.MockCreateDB(dbname, username)
}

func (p *MockPostgres) CreateSchema(db, role, schema string, logger logr.Logger) error {
	return p.MockCreateSchema(db, role, schema, logger)
}

func (p *MockPostgres) CreateExtension(db, extension string, logger logr.Logger) error {
	return p.MockCreateExtension(db, extension, logger)
}

func (p *MockPostgres) CreateGroupRole(role string) error {
	return p.MockCreateGroupRole(role)
}

func (p *MockPostgres) CreateUserRole(role, password string) (string, error) {
	return p.MockCreateUserRole(role, password)
}

func (p *MockPostgres) UpdatePassword(role, password string) error {
	return p.MockUpdatePassword(role, password)
}

func (p *MockPostgres) GrantRole(role, grantee string) error {
	return p.MockGrantRole(role, grantee)
}

func (p *MockPostgres) SetSchemaPrivileges(db, creator, role, schema, privs string, logger logr.Logger) error {
	return p.MockSetSchemaPrivileges(db, creator, role, schema, privs, logger)
}

func (p *MockPostgres) RevokeRole(role, revoked string) error {
	return p.MockRevokeRole(role, revoked)
}

func (p *MockPostgres) AlterDefaultLoginRole(role, setRole string) error {
	return p.MockAlterDefaultLoginRole(role, setRole)
}

func (p *MockPostgres) DropDatabase(db string, logger logr.Logger) error {
	return p.MockDropDatabase(db, logger)
}

func (p *MockPostgres) DropRole(role, newOwner, database string, logger logr.Logger) error {
	return p.MockDropRole(role, newOwner, database, logger)
}

func (p *MockPostgres) GetUser() string {
	return p.MockGetUser()
}
