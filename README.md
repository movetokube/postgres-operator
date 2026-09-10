# External PostgreSQL Server Operator for Kubernetes

[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/ext-postgres-operator)](https://artifacthub.io/packages/search?repo=ext-postgres-operator)
[![Sponsor](https://img.shields.io/badge/Sponsor_on_GitHub-ff69b4?style=for-the-badge&logo=github)](https://github.com/sponsors/hitman99)

Manage external PostgreSQL databases in Kubernetes with ease—supporting AWS RDS, Azure Database for PostgreSQL, GCP Cloud SQL, and more.

---

## Table of Contents

- [Sponsors](#sponsors)
- [Features](#features)
- [Supported Cloud Providers](#supported-cloud-providers)
- [Configuration](#configuration)
- [Installation](#installation)
- [Custom Resources (CRs)](#custom-resources-crs)
- [Multiple Operator Support](#multiple-operator-support)
- [Secret Templating](#secret-templating)
- [Compatibility](#compatibility)
- [Contributing](#contributing)
- [License](#license)

---

## Sponsors

Please consider supporting this project!

**Current Sponsors:**
_None yet. [Become a sponsor!](https://github.com/sponsors/hitman99)_

## Features

- Create databases and roles using Kubernetes CRs
- Automatic creation of randomized usernames and passwords
- Supports multiple user roles per database
- Auto-generates Kubernetes secrets with PostgreSQL connection URIs
- Supports AWS RDS, Azure Database for PostgreSQL, and GCP Cloud SQL
- Handles CRs in dynamically created namespaces
- Customizable secret values using templates

## AWS Specific Features when cloud_provider: "AWS"

- Enable IAM authentication for this user (PostgreSQL on AWS RDS only)

  ````yaml
  kind: PostgresUser
  ....
  spec:
    aws:
      enableIamAuth: false # (by Default false)
      ```
  ````

- AWS PG_repack extension installation / properly alter privileges for the owner user if cloud_provider: "AWS"

  ```yaml
  kind: Postgres
  ---
  spec:
    extensions:
      - pg_repack
  ```

---

## Supported Cloud Providers

### AWS

Set `POSTGRES_CLOUD_PROVIDER` to `AWS` via environment variable, Kubernetes Secret, or deployment manifest (`operator.yaml`).

### Azure Database for PostgreSQL – Flexible Server

> **Note:** Azure Single Server is deprecated as of v2.x. Only Flexible Server is supported.

- `POSTGRES_CLOUD_PROVIDER=Azure`
- `POSTGRES_DEFAULT_DATABASE=postgres`

### GCP

- `POSTGRES_CLOUD_PROVIDER=GCP`
- Configure a PostgreSQL connection secret
- Manually create a Master role and reference it in your CRs
- Master roles are never dropped by the operator

## Configuration

Set environment variables in [`config/manager/operator.yaml`](config/manager/operator.yaml):

| Name                | Description                                                    | Default          |
| ------------------- | -------------------------------------------------------------- | ---------------- |
| `WATCH_NAMESPACE`   | Namespace to watch. Empty string = all namespaces.             | (all namespaces) |
| `POSTGRES_INSTANCE` | Operator identity for multi-instance deployments.              | (empty)          |
| `KEEP_SECRET_NAME`  | Use user-provided secret names instead of auto-generated ones. | disabled         |

> **Note:**
> If enabling `KEEP_SECRET_NAME`, ensure there are no secret name conflicts in your namespace to avoid reconcile loops.

## Installation

### Install Using Helm (Recommended)

The Helm chart for this operator is located in the `charts/ext-postgres-operator` subdirectory. Follow these steps to install:

1. Add the Helm repository:

   ```bash
   helm repo add ext-postgres-operator https://movetokube.github.io/postgres-operator/
   ```

2. Install the operator:

   ```bash
   helm install -n operators ext-postgres-operator ext-postgres-operator/ext-postgres-operator
   ```

3. Customize the installation by modifying the values in [values.yaml](charts/ext-postgres-operator/values.yaml).

### Install Using Kustomize

This operator requires a Kubernetes Secret to be created in the same namespace as the operator itself.
The Secret should contain these keys: `POSTGRES_HOST`, `POSTGRES_USER`, `POSTGRES_PASS`, `POSTGRES_URI_ARGS`, `POSTGRES_CLOUD_PROVIDER`, `POSTGRES_DEFAULT_DATABASE`.

Example:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: ext-postgres-operator
  namespace: operators
type: Opaque
data:
  POSTGRES_HOST: cG9zdGdyZXM=
  POSTGRES_USER: cG9zdGdyZXM=
  POSTGRES_PASS: YWRtaW4=
  POSTGRES_URI_ARGS: IA==
  POSTGRES_CLOUD_PROVIDER: QVdT
  POSTGRES_DEFAULT_DATABASE: cG9zdGdyZXM=
```

To install the operator using Kustomize, follow these steps:

1. Configure Postgres credentials for the operator in `config/default/secret.yaml`.

2. Deploy the operator:

   ```bash
   kubectl kustomize config/default/ | kubectl apply -f -
   ```

   Alternatively, use [Kustomize](https://github.com/kubernetes-sigs/kustomize) directly:

   ```bash
   kustomize build config/default/ | kubectl apply -f -
   ```

## Custom Resources (CRs)

### Postgres

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
  schemas: # List of schemas the operator should create in database (optional)
    - stores
    - customers
  extensions: # List of extensions that should be created in the database (optional)
    - fuzzystrmatch
    - pgcrypto
```

This creates a database called `test-db` and a role `test-db-group` that is set as the owner of the database.
Reader and writer roles are also created. These roles have read and write permissions to all tables in the schemas created by the operator, if any.

### Scheduled permission repair

Permission repair is optional per `Postgres`. It adds missing grants; it never
revokes custom grants, changes ownership, recreates roles or rotates credentials.

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

### PostgresUser

```yaml
apiVersion: db.movetokube.com/v1alpha1
kind: PostgresUser
metadata:
  name: my-db-user
  namespace: app
  annotations:
    # OPTIONAL
    # use this to target which instance of operator should process this CR. See general config
    postgres.db.movetokube.com/instance: POSTGRES_INSTANCE
spec:
  # Keep the existing database, masterRole and schema configuration.
  permissionRepair:
    schedule: "0 2 * * *" # Five cron fields; every day at 02:00 UTC
    windowDuration: "30m" # Latest allowed start/end; defaults to 30m
    timeout: "5m" # Maximum transaction duration; defaults to 5m
  role: username
  database: my-db # This references the Postgres CR
  secretName: my-secret
  privileges: OWNER # Can be OWNER/READ/WRITE
  annotations: # Annotations to be propagated to the secrets metadata section (optional)
    foo: "bar"
  labels:
    foo: "bar" # Labels to be propagated to the secrets metadata section (optional)
  secretTemplate: # Output secrets can be customized using standard Go templates
    PQ_URL: "host={{.Host}} user={{.Role}} password={{.Password}} dbname={{.Database}}"
```

This creates a user role `username-<hash>` and grants role `test-db-group`, `test-db-writer` or `test-db-reader` depending on `privileges` property. Its credentials are put in secret `my-secret-my-db-user` (unless `KEEP_SECRET_NAME` is enabled).

`PostgresUser` needs to reference a `Postgres` in the same namespace.

Two `Postgres` referencing the same database can exist in more than one namespace. The last CR referencing a database will drop the group role and transfer database ownership to the role used by the operator.
Every PostgresUser has a generated Kubernetes secret attached to it, which contains the following data (i.e.):

| Key                 | Comment                                                                                                                                                                                    |
| ------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `DATABASE_NAME`     | Name of the database, same as in `Postgres` CR, copied for convenience                                                                                                                     |
| `HOST`              | PostgreSQL server host (including port number)                                                                                                                                             |
| `URI_ARGS`          | URI Args, same as in `Postgres` CR, copied for convenience                                                                                                                                 |
| `PASSWORD`          | Autogenerated password for user                                                                                                                                                            |
| `ROLE`              | Autogenerated role with login enabled (user)                                                                                                                                               |
| `LOGIN`             | Same as `ROLE`. In case `POSTGRES_CLOUD_PROVIDER` is set to "Azure", `LOGIN` it will be set to `{role}@{serverName}`, serverName is extracted from `POSTGRES_USER` from operator's config. |
| `POSTGRES_URL`      | Connection string for Posgres, could be used for Go applications                                                                                                                           |
| `POSTGRES_JDBC_URL` | JDBC compatible Postgres URI, formatter as `jdbc:postgresql://{POSTGRES_HOST}/{DATABASE_NAME}`                                                                                             |
| `HOSTNAME`          | The PostgreSQL server hostname (without port)                                                                                                                                              |
| `PORT`              | The PostgreSQL server port                                                                                                                                                                 |

| Functions      | Meaning                                                       |
| -------------- | ------------------------------------------------------------- |
| `mergeUriArgs` | Merge any provided uri args with any set in the `Postgres` CR |

### Multiple operator support

Run multiple operator instances by setting unique POSTGRES_INSTANCE values and using annotations in your CRs to assign them.

#### Annotations Use Case

With the help of annotations it is possible to create annotation-based copies of secrets in other namespaces.

For more information and an example, see [kubernetes-replicator#pull-based-replication](https://github.com/mittwald/kubernetes-replicator#pull-based-replication)

### Secret Templating

Users can specify the structure and content of secrets based on their unique requirements using standard
[Go templates](https://pkg.go.dev/text/template#hdr-Actions). This flexibility allows for a more tailored approach to
meeting the specific needs of different applications.

Available context:

| Variable    | Meaning                      |
| ----------- | ---------------------------- |
| `.Host`     | Database host                |
| `.Role`     | Generated user/role name     |
| `.Database` | Referenced database name     |
| `.Password` | Generated role password      |
| `.Hostname` | Database host (without port) |
| `.Port`     | Database port                |

### Compatibility

Postgres operator uses Operator SDK, which uses kubernetes client. Kubernetes client compatibility with Kubernetes cluster
can be found [here](https://github.com/kubernetes/client-go/blob/master/README.md#compatibility-matrix)

Postgres operator compatibility with Operator SDK version is in the table below

|                           | Operator SDK version | apiextensions.k8s.io |
| ------------------------- | -------------------- | -------------------- |
| `postgres-operator 0.4.x` | v0.17                | v1beta1              |
| `postgres-operator 1.x.x` | v0.18                | v1                   |
| `postgres-operator 2.x.x` | v1.39                | v1                   |
| `HEAD`                    | v1.39                | v1                   |

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md)

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
