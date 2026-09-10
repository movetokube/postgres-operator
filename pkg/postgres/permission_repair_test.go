package postgres

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"
)

func TestPermissionRepairValidation(t *testing.T) {
	cases := []PermissionRepair{
		{}, {Database: "db", Owner: "o", Reader: "r", Writer: "w"},
		{Database: "db", Owner: "o", Reader: "o", Writer: "w", Schemas: []string{"public"}},
		{Database: "db", Owner: "o", Reader: "r", Writer: "w", Schemas: []string{"pg_catalog"}},
		{Database: "db", Owner: "o", Reader: "r", Writer: "w", Schemas: []string{"information_schema"}},
		{Database: "db", Owner: "o", Reader: "r", Writer: "w", Schemas: []string{"pg_temp_2"}},
		{Database: "db", Owner: "o", Reader: "r", Writer: "w", Schemas: []string{""}},
		{Database: "db", Owner: "o\x00", Reader: "r", Writer: "w", Schemas: []string{"public"}},
	}
	for _, p := range cases {
		if err := repairPermissions(context.Background(), nil, p); err == nil {
			t.Fatalf("accepted %+v", p)
		}
	}
}
func TestPermissionRepairErrorsAreSafe(t *testing.T) {
	for _, err := range []error{errors.New("postgresql://admin:secret@example/db"), &pq.Error{Code: "42501", Message: "secret"}, context.Canceled, context.DeadlineExceeded} {
		got := PermissionRepairError(err)
		if got == "" || strings.Contains(got, "secret") {
			t.Fatalf("unsafe diagnostic %q", got)
		}
	}
}
func TestPermissionRepairRollbackOnGrantFailure(t *testing.T) {
	db, m, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m.ExpectBegin()
	m.ExpectQuery("SELECT pg_try_advisory").WillReturnRows(sqlmock.NewRows([]string{"locked"}).AddRow(true))
	m.ExpectExec("SET LOCAL ROLE").WillReturnResult(sqlmock.NewResult(0, 0))
	m.ExpectQuery("SELECT EXISTS").WillReturnRows(sqlmock.NewRows([]string{"invalid"}).AddRow(false))
	m.ExpectExec("GRANT CONNECT").WillReturnResult(sqlmock.NewResult(0, 0))
	m.ExpectExec("GRANT USAGE ON SCHEMA").WillReturnError(&pq.Error{Code: "42501"})
	m.ExpectRollback()
	if err := repairPermissions(context.Background(), db, PermissionRepair{Database: "db", Owner: "o", Reader: "r", Writer: "w", Schemas: []string{"public"}}); err == nil {
		t.Fatal("failure reported as success")
	}
	if err := m.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
