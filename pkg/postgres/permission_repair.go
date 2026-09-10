package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/lib/pq"
)

// PermissionRepair uses existing roles and only the explicitly configured schemas.
type PermissionRepair struct {
	Database, Owner, Reader, Writer string
	Schemas                         []string
}

// PermissionRepairError deliberately excludes connection strings and server detail.
func PermissionRepairError(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "permission repair timed out"
	}
	if errors.Is(err, context.Canceled) {
		return "permission repair canceled"
	}
	var pgErr *pq.Error
	if errors.As(err, &pgErr) {
		return "permission repair failed (SQLSTATE " + string(pgErr.Code) + ")"
	}
	return "permission repair failed; check database connectivity, configured schemas and owner privileges"
}

func (c *pg) RepairPermissions(ctx context.Context, repair PermissionRepair) error {
	// Use a URL encoder and PingContext: both connection setup and SQL are bounded.
	uri := &url.URL{Scheme: "postgresql", User: url.UserPassword(c.user, c.pass), Host: c.host, Path: "/" + repair.Database, RawQuery: c.args}
	db, err := sql.Open("postgres", uri.String())
	if err != nil {
		return err
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		return err
	}
	return repairPermissions(ctx, db, repair)
}

func repairPermissions(ctx context.Context, db *sql.DB, repair PermissionRepair) error {
	if repair.Database == "" || repair.Owner == "" || repair.Reader == "" || repair.Writer == "" || len(repair.Schemas) == 0 {
		return fmt.Errorf("database, all roles and at least one schema are required")
	}
	if repair.Owner == repair.Reader || repair.Owner == repair.Writer || repair.Reader == repair.Writer {
		return fmt.Errorf("repair roles must be distinct")
	}
	for _, name := range append([]string{repair.Database, repair.Owner, repair.Reader, repair.Writer}, repair.Schemas...) {
		if strings.ContainsRune(name, 0) {
			return fmt.Errorf("identifiers cannot contain NUL")
		}
	}
	for _, schema := range repair.Schemas {
		if schema == "" || schema == "information_schema" || strings.HasPrefix(schema, "pg_") {
			return fmt.Errorf("system or empty schema cannot be repaired")
		}
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Transaction-scoped lock also prevents overlap between CRs referring to one database.
	var locked bool
	if err := tx.QueryRowContext(ctx, "SELECT pg_try_advisory_xact_lock(hashtextextended(current_database(), 716913))").Scan(&locked); err != nil {
		return err
	}
	if !locked {
		return fmt.Errorf("another permission repair is running")
	}
	q := pq.QuoteIdentifier
	exec := func(statement string) error { _, err := tx.ExecContext(ctx, statement); return err }
	if err := exec("SET LOCAL ROLE " + q(repair.Owner)); err != nil {
		return err
	}
	// Fail instead of silently accepting GRANT warnings on objects belonging to another owner.
	var invalid bool
	if err := tx.QueryRowContext(ctx, repairOwnershipCheck, pq.Array(repair.Schemas)).Scan(&invalid); err != nil {
		return err
	}
	if invalid {
		return fmt.Errorf("configured schemas contain application objects owned by another role")
	}
	if err := exec(fmt.Sprintf("GRANT CONNECT ON DATABASE %s TO %s, %s", q(repair.Database), q(repair.Reader), q(repair.Writer))); err != nil {
		return err
	}
	for _, schema := range repair.Schemas {
		for _, statement := range []string{
			fmt.Sprintf("GRANT USAGE ON SCHEMA %s TO %s, %s", q(schema), q(repair.Reader), q(repair.Writer)),
			fmt.Sprintf("GRANT CREATE ON SCHEMA %s TO %s", q(schema), q(repair.Writer)),
		} {
			if err := exec(statement); err != nil {
				return err
			}
		}
	}
	// Enumerate objects to avoid extensions, internal routines and automatic types.
	// Multiranges inherit range ACLs; arrays and table row types use their parent ACLs.
	rows, err := tx.QueryContext(ctx, repairObjectGrants, pq.Array(repair.Schemas), repair.Reader, repair.Writer)
	if err != nil {
		return err
	}
	var statements []string
	for rows.Next() {
		var statement string
		if err := rows.Scan(&statement); err != nil {
			rows.Close()
			return err
		}
		statements = append(statements, statement)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, statement := range statements {
		if err := exec(statement); err != nil {
			return err
		}
	}
	// Only the stable owner creates migration objects. Do not change administrator defaults.
	for _, schema := range repair.Schemas {
		for _, grant := range []struct{ objects, privileges, role string }{
			{"TABLES", "SELECT", repair.Reader}, {"TABLES", "SELECT, INSERT, UPDATE, DELETE", repair.Writer},
			{"SEQUENCES", "USAGE, SELECT", repair.Writer}, {"ROUTINES", "EXECUTE", repair.Writer},
			{"TYPES", "USAGE", repair.Reader}, {"TYPES", "USAGE", repair.Writer},
		} {
			if err := exec(fmt.Sprintf("ALTER DEFAULT PRIVILEGES FOR ROLE %s IN SCHEMA %s GRANT %s ON %s TO %s", q(repair.Owner), q(schema), grant.privileges, grant.objects, q(grant.role))); err != nil {
				return err
			}
		}
	}
	// Large objects are database-local, not schema-local. Their defaults require PG18.
	var version int
	if err := tx.QueryRowContext(ctx, "SELECT current_setting('server_version_num')::int").Scan(&version); err != nil {
		return err
	}
	if version >= 180000 {
		if err := exec(fmt.Sprintf("ALTER DEFAULT PRIVILEGES FOR ROLE %s GRANT SELECT ON LARGE OBJECTS TO %s", q(repair.Owner), q(repair.Reader))); err != nil {
			return err
		}
		if err := exec(fmt.Sprintf("ALTER DEFAULT PRIVILEGES FOR ROLE %s GRANT SELECT, UPDATE ON LARGE OBJECTS TO %s", q(repair.Owner), q(repair.Writer))); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// System and extension objects retain their own privilege policy. Non-extension
// application objects must belong to the stable owner before adoption is repaired.
const repairOwnershipCheck = `SELECT EXISTS (
 SELECT 1 FROM (
  SELECT 'pg_class'::regclass AS classid, c.oid, c.relowner AS owner FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname=ANY($1) AND c.relkind IN ('r','p','v','m','f','S')
  UNION ALL SELECT 'pg_proc'::regclass, p.oid, p.proowner FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname=ANY($1)
  UNION ALL SELECT 'pg_type'::regclass, t.oid, t.typowner FROM pg_type t JOIN pg_namespace n ON n.oid=t.typnamespace LEFT JOIN pg_class c ON c.oid=t.typrelid WHERE n.nspname=ANY($1) AND t.typisdefined AND t.typtype NOT IN ('p','m') AND (t.typrelid=0 OR c.relkind='c') AND NOT EXISTS (SELECT 1 FROM pg_type a WHERE a.typarray=t.oid)
 ) obj WHERE owner<>current_user::regrole AND NOT EXISTS (SELECT 1 FROM pg_depend d WHERE d.classid=obj.classid AND d.objid=obj.oid AND (d.deptype='e' OR (obj.classid='pg_proc'::regclass AND d.deptype='i')))
) OR EXISTS (SELECT 1 FROM pg_database WHERE datname=current_database() AND datdba<>current_user::regrole) OR EXISTS (SELECT 1 FROM pg_namespace WHERE nspname=ANY($1) AND nspowner NOT IN (current_user::regrole, 'pg_database_owner'::regrole))`

const repairObjectGrants = `SELECT statement FROM (
 SELECT format('GRANT SELECT ON TABLE %I.%I TO %I; GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE %I.%I TO %I', n.nspname,c.relname,$2::text,n.nspname,c.relname,$3::text) AS statement
 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
 WHERE n.nspname=ANY($1) AND c.relowner=current_user::regrole AND c.relkind IN ('r','p','v','m','f') AND NOT EXISTS (SELECT 1 FROM pg_depend d WHERE d.classid='pg_class'::regclass AND d.objid=c.oid AND d.deptype='e')
 UNION ALL SELECT format('GRANT USAGE, SELECT ON SEQUENCE %I.%I TO %I',n.nspname,c.relname,$3::text)
 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
 WHERE n.nspname=ANY($1) AND c.relowner=current_user::regrole AND c.relkind='S' AND NOT EXISTS (SELECT 1 FROM pg_depend d WHERE d.classid='pg_class'::regclass AND d.objid=c.oid AND d.deptype='e')
 UNION ALL SELECT format('GRANT EXECUTE ON ROUTINE %I.%I(%s) TO %I',n.nspname,p.proname,pg_get_function_identity_arguments(p.oid),$3::text)
 FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace
 WHERE n.nspname=ANY($1) AND p.proowner=current_user::regrole AND NOT EXISTS (SELECT 1 FROM pg_depend d WHERE d.classid='pg_proc'::regclass AND d.objid=p.oid AND d.deptype IN ('e','i'))
 UNION ALL SELECT format('GRANT USAGE ON TYPE %I.%I TO %I, %I',n.nspname,t.typname,$2::text,$3::text)
 FROM pg_type t JOIN pg_namespace n ON n.oid=t.typnamespace LEFT JOIN pg_class c ON c.oid=t.typrelid
 WHERE n.nspname=ANY($1) AND t.typowner=current_user::regrole AND t.typisdefined AND t.typtype NOT IN ('p','m') AND (t.typrelid=0 OR c.relkind='c') AND NOT EXISTS (SELECT 1 FROM pg_type a WHERE a.typarray=t.oid) AND NOT EXISTS (SELECT 1 FROM pg_depend d WHERE d.classid='pg_type'::regclass AND d.objid=t.oid AND d.deptype='e')
 UNION ALL SELECT format('GRANT SELECT ON LARGE OBJECT %s TO %I; GRANT SELECT, UPDATE ON LARGE OBJECT %s TO %I',oid,$2::text,oid,$3::text) FROM pg_largeobject_metadata WHERE lomowner=current_user::regrole
) grants ORDER BY statement`
