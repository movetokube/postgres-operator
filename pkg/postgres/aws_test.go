package postgres

import (
	"database/sql"
	"fmt"
	"regexp"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestApplyPgRepackPrivileges(t *testing.T) {
	originalGetConnection := awsGetConnection
	defer func() {
		awsGetConnection = originalGetConnection
	}()

	mainDB, mainMock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create main sqlmock: %v", err)
	}
	defer mainDB.Close()

	tmpDB, tmpMock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create tmp sqlmock: %v", err)
	}
	defer tmpDB.Close()

	dbname := "test-db-dev"
	owner := "test-db-dev-owner"

	mainMock.ExpectQuery(regexp.QuoteMeta(fmt.Sprintf(GET_DB_OWNER, dbname))).
		WillReturnRows(sqlmock.NewRows([]string{"pg_get_userbyid"}).AddRow(owner))

	awsGetConnection = func(user, password, host, database, uriArgs string) (*sql.DB, error) {
		if database != dbname {
			t.Fatalf("expected database %s, got %s", dbname, database)
		}
		return tmpDB, nil
	}

	tmpMock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf(AWS_ALTER_REPACK_DEFAULT_PRIVS_TABLES, owner))).
		WillReturnResult(sqlmock.NewResult(0, 0))
	tmpMock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf(AWS_ALTER_REPACK_DEFAULT_PRIVS_SEQUENCES, owner))).
		WillReturnResult(sqlmock.NewResult(0, 0))

	c := &awspg{
		pg: pg{
			db:   mainDB,
			host: "localhost:5432",
			user: "postgres",
			pass: "postgres",
			args: "sslmode=disable",
		},
	}

	if err := c.applyPgRepackPrivileges(dbname); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if err := mainMock.ExpectationsWereMet(); err != nil {
		t.Fatalf("main DB expectations were not met: %v", err)
	}
	if err := tmpMock.ExpectationsWereMet(); err != nil {
		t.Fatalf("tmp DB expectations were not met: %v", err)
	}
}
