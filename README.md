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
- [Dedicated Operator Role](#dedicated-operator-role)
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

## Dedicated Operator Role

The operator connects to PostgreSQL using the credentials configured via the `POSTGRES_*` environment variables / Secret (see below). In many setups these credentials are the _server admin_ or _master user_.

You can also run the operator using a **dedicated operator login role** (recommended for production), for better separation of duties and easier auditing/rotation.

### What privileges are required?

This operator manages databases and roles, and also runs some operations inside the created databases. Your operator login role must be able to:

- Create databases and set database owners (`CREATE DATABASE`, `ALTER DATABASE ... OWNER TO ...`)
- Grant database-level privileges (the operator runs `GRANT CREATE ON DATABASE ...`)
- Create roles/users and manage role membership (`CREATE ROLE`, `DROP ROLE`, `GRANT <role> TO <grantee>`, `REVOKE ...`)
- Connect to managed databases and:
  - Create schemas (`CREATE SCHEMA ... AUTHORIZATION ...`)
  - Create extensions (`CREATE EXTENSION ...`)
  - Grant privileges / alter default privileges within schemas

The operator also grants each created role to itself, so it can later revoke privileges, reassign ownership, and drop roles cleanly.

### Example: creating an operator role

The exact SQL depends on how your PostgreSQL instance is managed. In plain PostgreSQL (self-hosted), you can often do something like:

```sql
-- Create a dedicated login for the Kubernetes operator
CREATE ROLE pgoperator WITH
 PASSWORD 'YourSecurePassword123!'
 LOGIN
 CREATEDB
 CREATEROLE;
```

For managed services, you typically create `ext_postgres_operator` while connected as the platform-provided admin and grant only the capabilities supported by that platform.

### Cloud provider notes

Because this is an _external / managed PostgreSQL_ operator, the feasibility of least-privilege depends on your provider.

- **AWS RDS (PostgreSQL)**
  - The initial “master user” is a member of the `rds_superuser` role.
  - A dedicated operator role is usually possible: create a login role with `CREATEDB`/`CREATEROLE`, then grant it any extra permissions you need for extensions/schemas.
  - Docs: <https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/Appendix.PostgreSQL.CommonDBATasks.Roles.html>

- **GCP Cloud SQL (PostgreSQL)**
  - Cloud SQL does not expose true `SUPERUSER`. The default `postgres` user is a member of `cloudsqlsuperuser` and has `CREATEROLE` and `CREATEDB`.
  - You can create other users/roles with reduced privileges (for example, an operator role with `CREATEROLE`/`CREATEDB`), but some operations (notably certain extensions) may require `cloudsqlsuperuser`.
  - Docs: <https://cloud.google.com/sql/docs/postgres/users>

- **Azure Database for PostgreSQL  Flexible Server**
  - The server admin user is a member of `azure_pg_admin` and has `CREATEDB` and `CREATEROLE`; the `azuresu` superuser role is reserved for Microsoft.
  - A dedicated operator role is supported: create a user with `CREATEDB`/`CREATEROLE`, optionally add it to `azure_pg_admin` if you need additional capabilities.
  - Docs: <https://learn.microsoft.com/en-us/azure/postgresql/flexible-server/security-manage-database-users>

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
