# Scheduled permission repair

Permission repair is optional per `Postgres`. It adds missing grants; it never
revokes custom grants, changes ownership, recreates roles or rotates credentials.

What is covered by permissionRepair:

- All application schemas.
- Tables, partitions, views, materialized views, and foreign tables.
- Sequences.
- Functions and procedures.
- Owner-managed types, domains, and large objects.
- Default privileges for future objects created by the owner role.

Note: Time format is UTC

This is part of Postgres object spec

```yaml
apiVersion: db.movetokube.com/v1alpha1
kind: Postgres
metadata:
  name: my-db
  namespace: app
  annotations:
    # OPTIONAL
    # use this to target which instance of operator should process this CR. See General config
    postgres.db.movetokube.com/instance: POSTGRES_INSTANCE
spec:
  database: test-db # Name of database created in PostgreSQL
  dropOnDelete: false # Set to true if you want the operator to drop the database and role when this CR is deleted (optional)
  masterRole: test-db-group (optional)
  permissionRepair: # If that field is omitted permissionRepair will be disabled
    schedule: "0 2 * * *" # Five cron fields; every day at 02:00 UTC
    windowDuration: "30m" # Latest allowed start/end; defaults to 30m
    timeout: "5m" # Maximum transaction duration; defaults to 5m
  schemas: # List of schemas the operator should create in database (optional)
    - stores
    - customers
  extensions: # List of extensions that should be created in the database (optional)
    - fuzzystrmatch
    - pgcrypto
```

The schedule accepts five-field cron syntax Schedules always run in UTC.

With a schedule configured, initial provisioning still applies normal grants.
Subsequent permission repair runs only in its window, using `spec.schemas` and the
existing roles in `status.roles`. Keep the full list of application schemas in
`spec.schemas`; system schemas are rejected. Removing `permissionRepair` restores
the original event-driven schema-grant behavior.

Repair covers database `CONNECT`, schema `USAGE` (and writer `CREATE`), tables,
partitions, views, materialized views, foreign tables, sequences, functions,
procedures, types/domains and owner-managed large objects. Reader gets table/large
object `SELECT` and type `USAGE`. Writer gets table CRUD, sequence `USAGE, SELECT`,
routine `EXECUTE`, type `USAGE`, and large-object `SELECT, UPDATE`. It does not grant
reader execution of routines or writer `TRUNCATE`/ownership rights.

Existing application objects must belong to the stable owner. Extension-managed objects and PostgreSQL-generated internal routines (such as
range constructors) retain their existing policy. RLS policies, foreign-server credentials
and user mappings are not repaired. Large objects are scoped by owner in the
current database because they do not belong to schemas.

Default privileges target the stable owner explicitly, in each configured schema.
They therefore apply to migration objects created as that role, not objects
created as an administrator or another login/group. Large-object defaults require
PostgreSQL 18; earlier servers receive existing large-object grants only. New
schemas must be added to `spec.schemas`. Reconcile extensions through their normal
configuration.

The controller persists `nextRunTime`, `lastAttemptTime`, `lastSuccessTime` and
`error` in `status.permissionRepair`. Provisioning `status.succeeded` stays separate
from repair failures. Enabling/changing the schedule (or its database/schema/role
scope) schedules the next future occurrence. After a restart, a pending occurrence
runs only if still inside its window; expired occurrences are skipped. Each
occurrence is claimed before SQL, attempted at most once, and failures wait until
the next occurrence. A crash after claiming can skip that attempt; interrupted
attempts remain visible in status. This avoids immediate retry loops and repairs
outside maintenance windows.

Repairs run in a transaction, use cancellation/timeouts capped at the window end,
and take a PostgreSQL advisory lock to prevent concurrent repairs of the same
database. Run the operator with its default leader election enabled. Error status
never includes connection strings or passwords.

Upgrade the installed CRD as well as the controller before configuring the new
fields. Helm does not automatically upgrade CRDs from a chart's `crds/` directory.

To run the real privilege/rollback tests against a **disposable** PostgreSQL server:

```bash
PERMISSION_REPAIR_TEST_DSN='postgresql://postgres@127.0.0.1:55439/postgres?sslmode=disable' go test ./pkg/postgres -run TestPermissionRepairIntegration -count=1
```

The integration test creates and removes a temporary database and roles. CI runs
it on PostgreSQL 16 and 18; scheduling/controller tests require no live cluster.
