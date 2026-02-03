package postgres

import (
	"database/sql"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"
)

func TestDropRole_NoOpForReservedRole(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	p := &pg{db: db, user: "operator"}

	if err := p.DropRole("rdsadmin", "newowner", "mydb"); err != nil {
		t.Fatalf("DropRole: %v", err)
	}
}

func TestDropRole_ReassignsAndDrops(t *testing.T) {
	rootDB, rootMock, err := sqlmock.New()
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

	// Ensure operator can manage the role, and newOwner if needed.
	rootMock.ExpectExec(regexp.QuoteMeta(`GRANT "todelete" TO "operator"`)).WillReturnResult(sqlmock.NewResult(0, 1))
	rootMock.ExpectExec(regexp.QuoteMeta(`GRANT "newowner" TO "operator"`)).WillReturnResult(sqlmock.NewResult(0, 1))

	perMock.ExpectExec(regexp.QuoteMeta(`REASSIGN OWNED BY "todelete" TO "newowner"`)).WillReturnResult(sqlmock.NewResult(0, 1))
	perMock.ExpectExec(regexp.QuoteMeta(`DROP OWNED BY "todelete"`)).WillReturnResult(sqlmock.NewResult(0, 1))
	rootMock.ExpectExec(regexp.QuoteMeta(`DROP ROLE "todelete"`)).WillReturnResult(sqlmock.NewResult(0, 1))

	if err := p.DropRole("todelete", "newowner", "mydb"); err != nil {
		t.Fatalf("DropRole: %v", err)
	}

	if err := rootMock.ExpectationsWereMet(); err != nil {
		t.Fatalf("root expectations: %v", err)
	}
	if err := perMock.ExpectationsWereMet(); err != nil {
		t.Fatalf("per-db expectations: %v", err)
	}
}

func TestCreateGroupRole_GrantsRoleToOperatorUser(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	p := &pg{db: db, user: "operator"}

	mock.ExpectExec(regexp.QuoteMeta(`CREATE ROLE "myrole"`)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`GRANT "myrole" TO "operator"`)).WillReturnResult(sqlmock.NewResult(0, 1))

	if err := p.CreateGroupRole("myrole"); err != nil {
		t.Fatalf("CreateGroupRole: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestCreateUserRole_GrantsRoleToOperatorUser(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	p := &pg{db: db, user: "operator"}

	mock.ExpectExec(regexp.QuoteMeta(`CREATE ROLE "app-123" WITH LOGIN PASSWORD 'pass'`)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`GRANT "app-123" TO "operator"`)).WillReturnResult(sqlmock.NewResult(0, 1))

	role, err := p.CreateUserRole("app-123", "pass")
	if err != nil {
		t.Fatalf("CreateUserRole: %v", err)
	}
	if role != "app-123" {
		t.Fatalf("CreateUserRole role: got %q want %q", role, "app-123")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestCreateUserRole_IgnoresAlreadyMemberError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	p := &pg{db: db, user: "operator"}

	mock.ExpectExec(regexp.QuoteMeta(`CREATE ROLE "app-123" WITH LOGIN PASSWORD 'pass'`)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`GRANT "app-123" TO "operator"`)).WillReturnError(&pq.Error{Code: "0LP01"})

	role, err := p.CreateUserRole("app-123", "pass")
	if err != nil {
		t.Fatalf("CreateUserRole: %v", err)
	}
	if role != "app-123" {
		t.Fatalf("CreateUserRole role: got %q want %q", role, "app-123")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}
