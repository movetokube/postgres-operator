package postgres

import (
	"database/sql"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestCreateDB(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	p := &pg{db: db, user: "operator"}

	mock.ExpectExec(regexp.QuoteMeta(`CREATE DATABASE "mydb"`)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`ALTER DATABASE "mydb" OWNER TO "owner"`)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`GRANT CREATE ON DATABASE "mydb" TO "operator"`)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`GRANT CREATE ON DATABASE "mydb" TO "owner"`)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`GRANT CONNECT ON DATABASE "mydb" TO "operator"`)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`GRANT CONNECT ON DATABASE "mydb" TO "owner"`)).WillReturnResult(sqlmock.NewResult(0, 1))

	if err := p.CreateDB("mydb", "owner"); err != nil {
		t.Fatalf("CreateDB: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestRenameGroupRole(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	p := &pg{db: db}

	mock.ExpectExec(regexp.QuoteMeta(`ALTER ROLE "old" RENAME TO "new"`)).WillReturnResult(sqlmock.NewResult(0, 1))

	if err := p.RenameGroupRole("old", "new"); err != nil {
		t.Fatalf("RenameGroupRole: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestUpdatePassword(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	p := &pg{db: db}

	mock.ExpectExec(regexp.QuoteMeta(`ALTER ROLE "user" WITH PASSWORD 'newpass'`)).WillReturnResult(sqlmock.NewResult(0, 1))

	if err := p.UpdatePassword("user", "newpass"); err != nil {
		t.Fatalf("UpdatePassword: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestGrantRole(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	p := &pg{db: db}

	mock.ExpectExec(regexp.QuoteMeta(`GRANT "role" TO "grantee"`)).WillReturnResult(sqlmock.NewResult(0, 1))

	if err := p.GrantRole("role", "grantee"); err != nil {
		t.Fatalf("GrantRole: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestRevokeRole(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	p := &pg{db: db}

	mock.ExpectExec(regexp.QuoteMeta(`REVOKE "role" FROM "revoked"`)).WillReturnResult(sqlmock.NewResult(0, 1))

	if err := p.RevokeRole("role", "revoked"); err != nil {
		t.Fatalf("RevokeRole: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestAlterDefaultLoginRole(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	p := &pg{db: db}

	mock.ExpectExec(regexp.QuoteMeta(`ALTER USER "user" SET ROLE "group"`)).WillReturnResult(sqlmock.NewResult(0, 1))

	if err := p.AlterDefaultLoginRole("user", "group"); err != nil {
		t.Fatalf("AlterDefaultLoginRole: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestAlterDatabaseOwner(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	p := &pg{db: db}

	mock.ExpectExec(regexp.QuoteMeta(`ALTER DATABASE "mydb" OWNER TO "newowner"`)).WillReturnResult(sqlmock.NewResult(0, 1))

	if err := p.AlterDatabaseOwner("mydb", "newowner"); err != nil {
		t.Fatalf("AlterDatabaseOwner: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestAlterDatabaseOwner_EmptyOwnerNoop(t *testing.T) {
	p := &pg{}
	if err := p.AlterDatabaseOwner("mydb", ""); err != nil {
		t.Fatalf("AlterDatabaseOwner: %v", err)
	}
}

func TestDropDatabase(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	p := &pg{db: db}

	mock.ExpectExec(regexp.QuoteMeta(`REVOKE CONNECT ON DATABASE "mydb" FROM public`)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`DROP DATABASE "mydb" WITH (FORCE)`)).WillReturnResult(sqlmock.NewResult(0, 1))

	if err := p.DropDatabase("mydb"); err != nil {
		t.Fatalf("DropDatabase: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestReassignDatabaseOwner(t *testing.T) {
	rootDB, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New root: %v", err)
	}
	defer rootDB.Close()

	perDB, perMock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New per-db: %v", err)
	}
	defer perDB.Close()

	original := getConnection
	getConnection = func(user, password, host, database, uriArgs string) (*sql.DB, error) {
		return perDB, nil
	}
	t.Cleanup(func() { getConnection = original })

	p := &pg{db: rootDB, user: "operator", pass: "pass", host: "host", args: ""}

	perMock.ExpectExec(regexp.QuoteMeta(`REASSIGN OWNED BY "old" TO "new"`)).WillReturnResult(sqlmock.NewResult(0, 1))

	if err := p.ReassignDatabaseOwner("mydb", "old", "new"); err != nil {
		t.Fatalf("ReassignDatabaseOwner: %v", err)
	}
	if err := perMock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestReassignDatabaseOwner_NoOpWhenSame(t *testing.T) {
	p := &pg{}
	if err := p.ReassignDatabaseOwner("mydb", "owner", "owner"); err != nil {
		t.Fatalf("ReassignDatabaseOwner: %v", err)
	}
}

func TestCreateSchema(t *testing.T) {
	rootDB, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New root: %v", err)
	}
	defer rootDB.Close()

	perDB, perMock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New per-db: %v", err)
	}
	defer perDB.Close()

	original := getConnection
	getConnection = func(user, password, host, database, uriArgs string) (*sql.DB, error) {
		return perDB, nil
	}
	t.Cleanup(func() { getConnection = original })

	p := &pg{db: rootDB, user: "operator", pass: "pass", host: "host", args: ""}

	perMock.ExpectExec(regexp.QuoteMeta(`CREATE SCHEMA IF NOT EXISTS "myschema" AUTHORIZATION "owner"`)).WillReturnResult(sqlmock.NewResult(0, 1))

	if err := p.CreateSchema("mydb", "owner", "myschema"); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}
	if err := perMock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestCreateExtension(t *testing.T) {
	rootDB, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New root: %v", err)
	}
	defer rootDB.Close()

	perDB, perMock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New per-db: %v", err)
	}
	defer perDB.Close()

	original := getConnection
	getConnection = func(user, password, host, database, uriArgs string) (*sql.DB, error) {
		return perDB, nil
	}
	t.Cleanup(func() { getConnection = original })

	p := &pg{db: rootDB, user: "operator", pass: "pass", host: "host", args: ""}

	perMock.ExpectExec(regexp.QuoteMeta(`CREATE EXTENSION IF NOT EXISTS "pgcrypto"`)).WillReturnResult(sqlmock.NewResult(0, 1))

	if err := p.CreateExtension("mydb", "pgcrypto"); err != nil {
		t.Fatalf("CreateExtension: %v", err)
	}
	if err := perMock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestSetSchemaPrivileges(t *testing.T) {
	rootDB, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New root: %v", err)
	}
	defer rootDB.Close()

	perDB, perMock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New per-db: %v", err)
	}
	defer perDB.Close()

	original := getConnection
	getConnection = func(user, password, host, database, uriArgs string) (*sql.DB, error) {
		return perDB, nil
	}
	t.Cleanup(func() { getConnection = original })

	p := &pg{db: rootDB, user: "operator", pass: "pass", host: "host", args: ""}

	privs := PostgresSchemaPrivileges{
		DB:            "mydb",
		Role:          "app",
		Schema:        "public",
		Privs:         "SELECT",
		SequencePrivs: "",
		FunctionPrivs: "",
		CreateSchema:  true,
	}

	perMock.ExpectExec(regexp.QuoteMeta(`GRANT USAGE ON SCHEMA "public" TO "app"`)).WillReturnResult(sqlmock.NewResult(0, 1))
	perMock.ExpectExec(regexp.QuoteMeta(`GRANT SELECT ON ALL TABLES IN SCHEMA "public" TO "app"`)).WillReturnResult(sqlmock.NewResult(0, 1))
	perMock.ExpectExec(regexp.QuoteMeta(`ALTER DEFAULT PRIVILEGES IN SCHEMA "public" GRANT SELECT ON TABLES TO "app"`)).WillReturnResult(sqlmock.NewResult(0, 1))
	perMock.ExpectExec(regexp.QuoteMeta(`GRANT CREATE ON SCHEMA "public" TO "app"`)).WillReturnResult(sqlmock.NewResult(0, 1))

	if err := p.SetSchemaPrivileges(privs); err != nil {
		t.Fatalf("SetSchemaPrivileges: %v", err)
	}
	if err := perMock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}
