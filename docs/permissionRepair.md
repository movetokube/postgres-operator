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
spec:
  # Keep the existing database, masterRole and schema configuration.
  permissionRepair:
    schedule: "0 2 * * *" # Five cron fields; every day at 02:00 UTC
    windowDuration: "30m" # Latest allowed start/end; defaults to 30m
    timeout: "5m" # Maximum transaction duration; defaults to 5m
```

The schedule accepts five-field cron syntax (including lists, ranges and steps),
without seconds or `@daily`-style shortcuts. Schedules always use UTC, regardless
of the operator host timezone or daylight-saving changes. Timezone overrides in
the cron expression are rejected. `windowDuration` must be positive and at most `24h`; `timeout` must be
positive and no longer than the window. Invalid configuration is reported in
`status.permissionRepair.error` and no scheduled repair runs.

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
