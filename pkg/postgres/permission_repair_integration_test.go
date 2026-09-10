package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/lib/pq"
)

// Run only against a disposable PostgreSQL server; this test creates and drops its own database/roles.
func TestPermissionRepairIntegration(t *testing.T) {
	dsn := os.Getenv("PERMISSION_REPAIR_TEST_DSN")
	if dsn == "" {
		t.Skip("set PERMISSION_REPAIR_TEST_DSN to a disposable PostgreSQL server")
	}
	admin, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.Close() })
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	database := "repair_" + suffix
	owner := "owner_\"" + suffix
	reader := "reader_" + suffix
	writer := "writer_" + suffix
	q := pq.QuoteIdentifier
	mustExec := func(db *sql.DB, query string) {
		t.Helper()
		if _, err := db.Exec(query); err != nil {
			t.Fatal(err)
		}
	}
	for _, role := range []string{owner, reader, writer} {
		mustExec(admin, "CREATE ROLE "+q(role))
	}
	t.Cleanup(func() {
		if _, err := admin.Exec("DROP DATABASE IF EXISTS " + q(database) + " WITH (FORCE)"); err != nil {
			t.Error(err)
		}
		for _, role := range []string{owner, reader, writer} {
			if _, err := admin.Exec("DROP ROLE " + q(role)); err != nil {
				t.Error(err)
			}
		}
	})
	mustExec(admin, "CREATE DATABASE "+q(database)+" OWNER "+q(owner))
	uri, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	uri.Path = "/" + database
	target, err := sql.Open("postgres", uri.String())
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	target.SetMaxOpenConns(3)
	schema := "billing\"items"
	mustExec(target, "CREATE EXTENSION hstore; CREATE EXTENSION postgres_fdw; CREATE SERVER dummy FOREIGN DATA WRAPPER postgres_fdw")
	mustExec(target, "GRANT USAGE ON FOREIGN SERVER dummy TO "+q(owner))
	mustExec(target, "SET ROLE "+q(owner)+`; CREATE SCHEMA `+q(schema)+`; CREATE SCHEMA private;
 CREATE TABLE public.existing(id int, value text);
 CREATE TABLE public.partitioned(id int) PARTITION BY RANGE(id);
 CREATE TABLE public.partition_1 PARTITION OF public.partitioned FOR VALUES FROM (0) TO (10);
 CREATE VIEW public.a_view AS SELECT * FROM public.existing;
 CREATE MATERIALIZED VIEW public.a_matview AS SELECT * FROM public.existing;
 CREATE SEQUENCE public.counter;
 CREATE FUNCTION public.work(i int) RETURNS int LANGUAGE sql AS 'SELECT i';
 CREATE FUNCTION public.work(i text) RETURNS text LANGUAGE sql AS 'SELECT i';
 CREATE PROCEDURE public.proc() LANGUAGE sql AS 'SELECT 1';
 CREATE TYPE public.mood AS ENUM ('ok');
 CREATE DOMAIN public.positive AS int CHECK(VALUE>0);
 CREATE TYPE public.pair AS (a int,b text);
 CREATE TYPE public.custom_range AS RANGE (subtype=integer);
 CREATE FOREIGN TABLE public.foreign_t(id int) SERVER dummy;
 CREATE TABLE `+q(schema)+`.other(id int);
 CREATE TABLE private.hidden(id int);
 REVOKE ALL ON ALL ROUTINES IN SCHEMA public FROM PUBLIC;
 ALTER DEFAULT PRIVILEGES REVOKE EXECUTE ON ROUTINES FROM PUBLIC;
 ALTER DEFAULT PRIVILEGES REVOKE USAGE ON TYPES FROM PUBLIC;
 RESET ROLE;`)
	// Preserve custom grants, while repairing only the expected baseline.
	mustExec(target, "GRANT TRUNCATE ON public.existing TO "+q(writer))
	createLO := func() uint32 {
		t.Helper()
		tx, err := target.Begin()
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		if _, err = tx.Exec("SET LOCAL ROLE " + q(owner)); err != nil {
			t.Fatal(err)
		}
		var oid uint32
		if err = tx.QueryRow("SELECT lo_create(0)").Scan(&oid); err != nil {
			t.Fatal(err)
		}
		if err = tx.Commit(); err != nil {
			t.Fatal(err)
		}
		return oid
	}
	lo := createLO()
	repair := PermissionRepair{Database: database, Owner: owner, Reader: reader, Writer: writer, Schemas: []string{"public", schema}}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	password, _ := uri.User.Password()
	pgClient := &pg{host: uri.Host, user: uri.User.Username(), pass: password, args: uri.RawQuery}
	if err = pgClient.RepairPermissions(ctx, repair); err != nil {
		t.Fatal(err)
	}
	// Idempotent repair and custom grants must survive repeated runs.
	if err = repairPermissions(ctx, target, repair); err != nil {
		t.Fatal(err)
	}
	check := func(query string, want bool, args ...any) {
		t.Helper()
		var got bool
		if err := target.QueryRow(query, args...).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("%s args=%v: got %v want %v", query, args, got, want)
		}
	}
	for _, table := range []string{"public.existing", "public.partitioned", "public.partition_1", "public.a_view", "public.a_matview", "public.foreign_t", q(schema) + ".other"} {
		check("SELECT has_table_privilege($1,$2,'SELECT')", true, reader, table)
		for _, priv := range []string{"SELECT", "INSERT", "UPDATE", "DELETE"} {
			check("SELECT has_table_privilege($1,$2,$3)", true, writer, table, priv)
		}
		check("SELECT has_table_privilege($1,$2,'INSERT')", false, reader, table)
	}
	check("SELECT has_table_privilege($1,'public.existing','TRUNCATE')", true, writer)
	check("SELECT has_table_privilege($1,'private.hidden','SELECT')", false, reader)
	check("SELECT has_sequence_privilege($1,'public.counter','USAGE')", true, writer)
	check("SELECT has_sequence_privilege($1,'public.counter','UPDATE')", false, writer)
	for _, routine := range []string{"public.work(integer)", "public.work(text)", "public.proc()"} {
		check("SELECT has_function_privilege($1,$2,'EXECUTE')", true, writer, routine)
		check("SELECT has_function_privilege($1,$2,'EXECUTE')", false, reader, routine)
	}
	for _, typ := range []string{"public.mood", "public.positive", "public.pair", "public.custom_range", "public.custom_multirange"} {
		check("SELECT has_type_privilege($1,$2,'USAGE')", true, reader, typ)
	}
	check("SELECT EXISTS (SELECT 1 FROM pg_largeobject_metadata l, LATERAL aclexplode(l.lomacl) a WHERE l.oid=$2 AND a.grantee=$1::regrole AND a.privilege_type='SELECT')", true, reader, lo)
	check("SELECT EXISTS (SELECT 1 FROM pg_largeobject_metadata l, LATERAL aclexplode(l.lomacl) a WHERE l.oid=$2 AND a.grantee=$1::regrole AND a.privilege_type='UPDATE')", true, writer, lo)
	// Normal provisioning must also repair the stable owner's defaults.
	mustExec(target, "ALTER DEFAULT PRIVILEGES FOR ROLE "+q(owner)+" IN SCHEMA public REVOKE SELECT ON TABLES FROM "+q(reader))
	if err := pgClient.SetSchemaPrivileges(PostgresSchemaPrivileges{DB: database, Owner: owner, Role: reader, Schema: "public", Privs: "SELECT"}); err != nil {
		t.Fatal(err)
	}
	// Defaults must be on the owner, even though repair was invoked through an admin connection.
	mustExec(target, "SET ROLE "+q(owner)+`; CREATE TABLE public.future(id int); CREATE SEQUENCE public.future_seq;
 CREATE PROCEDURE public.future_proc() LANGUAGE sql AS 'SELECT 1'; CREATE TYPE public.future_type AS ENUM ('new'); RESET ROLE;`)
	check("SELECT has_table_privilege($1,'public.future','SELECT')", true, reader)
	check("SELECT has_table_privilege($1,'public.future','UPDATE')", true, writer)
	check("SELECT has_sequence_privilege($1,'public.future_seq','USAGE')", true, writer)
	check("SELECT has_function_privilege($1,'public.future_proc()','EXECUTE')", true, writer)
	check("SELECT has_type_privilege($1,'public.future_type','USAGE')", true, reader)
	var version int
	if err = target.QueryRow("SHOW server_version_num").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version >= 180000 {
		check("SELECT EXISTS (SELECT 1 FROM pg_largeobject_metadata l, LATERAL aclexplode(l.lomacl) a WHERE l.oid=$2 AND a.grantee=$1::regrole AND a.privilege_type='SELECT')", true, reader, createLO())
	}
	// Explicit drift repair.
	mustExec(target, "REVOKE SELECT ON public.existing FROM "+q(reader)+"; REVOKE USAGE ON public.counter FROM "+q(writer))
	if err = repairPermissions(ctx, target, repair); err != nil {
		t.Fatal(err)
	}
	check("SELECT has_table_privilege($1,'public.existing','SELECT')", true, reader)
	check("SELECT has_sequence_privilege($1,'public.counter','USAGE')", true, writer)
	// A second CR/operator cannot overlap while the transaction lock is held.
	tx, err := target.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(current_database(),716913))"); err != nil {
		t.Fatal(err)
	}
	err = repairPermissions(ctx, target, repair)
	tx.Rollback()
	if err == nil || !strings.Contains(err.Error(), "another permission repair") {
		t.Fatalf("lock: %v", err)
	}
	// A failed transaction cannot leave grants on earlier objects committed.
	mustExec(target, "REVOKE SELECT ON public.existing, public.future FROM "+q(reader))
	tx, err = target.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec("SELECT oid FROM pg_class WHERE oid = 'public.future'::regclass FOR UPDATE"); err != nil {
		t.Fatal(err)
	}
	short, cancelShort := context.WithTimeout(context.Background(), 100*time.Millisecond)
	err = repairPermissions(short, target, repair)
	cancelShort()
	tx.Rollback()
	if err == nil {
		t.Fatal("expected timeout")
	}
	check("SELECT has_table_privilege($1,'public.existing','SELECT')", false, reader)
	// Wrong ownership must not be silently treated as repaired.
	mustExec(target, "CREATE TABLE public.not_adopted(id int)")
	if err = repairPermissions(ctx, target, repair); err == nil {
		t.Fatal("expected ownership failure")
	}
}
