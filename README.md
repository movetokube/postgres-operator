# External PostgreSQL Server Operator for Kubernetes

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

[![Sponsor](https://img.shields.io/badge/Sponsor_on_GitHub-ff69b4?style=for-the-badge&logo=github)](https://github.com/sponsors/hitman99)

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

| Name | Description | Default |
| --- | --- | --- |
| `WATCH_NAMESPACE` | Namespace to watch. Empty string = all namespaces. | (all namespaces) |
| `POSTGRES_INSTANCE` | Operator identity for multi-instance deployments. | (empty) |
| `KEEP_SECRET_NAME` | Use user-provided secret names instead of auto-generated ones. | disabled |

> **Note:**
> If enabling `KEEP_SECRET_NAME`, ensure there are no secret name conflicts in your namespace to avoid reconcile loops.

## Installation

This operator requires a Kubernetes Secret to be created in the same namespace as operator itself.
Secret should contain these keys: POSTGRES_HOST, POSTGRES_USER, POSTGRES_PASS, POSTGRES_URI_ARGS, POSTGRES_CLOUD_PROVIDER, POSTGRES_DEFAULT_DATABASE.
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

To install the operator using kustomize, follow the steps below.

1. Configure Postgres credentials for the operator in `config/secret.yaml`
2. Create namespace if needed with\
   `kubectl apply -f config/namespace.yaml`
3. Apply the secret with\
   `kubectl apply -f deploy/secret.yaml`
4. Create the operator with either\
    `kubectl kustomize config/default/ | apply -f -`\
    or by using [kustomize](https://github.com/kubernetes-sigs/kustomize) directly\
    `kustomize build config/default/ | apply -f -`

Alternatively you can install operator using Helm Chart located in the
`charts/ext-postgres-operator` subdirectory. Sample installation commands provided below:

```
helm repo add ext-postgres-operator https://movetokube.github.io/postgres-operator/
helm install -n operators ext-postgres-operator ext-postgres-operator/ext-postgres-operator
```
See [values.yaml](charts/ext-postgres-operator/values.yaml) for the possible values to define.

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
  database: my-db       # This references the Postgres CR
  secretName: my-secret
  privileges: OWNER     # Can be OWNER/READ/WRITE
  annotations:          # Annotations to be propagated to the secrets metadata section (optional)
    foo: "bar"
  labels:
    foo: "bar"          # Labels to be propagated to the secrets metadata section (optional)
  secretTemplate:       # Output secrets can be customized using standard Go templates
    PQ_URL: "host={{.Host}} user={{.Role}} password={{.Password}} dbname={{.Database}}"
```

This creates a user role `username-<hash>` and grants role `test-db-group`, `test-db-writer` or `test-db-reader` depending on `privileges` property. Its credentials are put in secret `my-secret-my-db-user` (unless `KEEP_SECRET_NAME` is enabled).

`PostgresUser` needs to reference a `Postgres` in the same namespace.

Two `Postgres` referencing the same database can exist in more than one namespace. The last CR referencing a database will drop the group role and transfer database ownership to the role used by the operator.
Every PostgresUser has a generated Kubernetes secret attached to it, which contains the following data (i.e.):

|  Key                 | Comment             |
|----------------------|---------------------|
| `DATABASE_NAME`      | Name of the database, same as in `Postgres` CR, copied for convenience |
| `HOST`               | PostgreSQL server host |
| `PASSWORD`           | Autogenerated password for user |
| `ROLE`               | Autogenerated role with login enabled (user) |
| `LOGIN`              | Same as `ROLE`. In case `POSTGRES_CLOUD_PROVIDER` is set to "Azure", `LOGIN` it will be set to `{role}@{serverName}`, serverName is extracted from `POSTGRES_USER` from operator's config. |
| `POSTGRES_URL`       | Connection string for Posgres, could be used for Go applications |
| `POSTGRES_JDBC_URL`  | JDBC compatible Postgres URI, formatter as `jdbc:postgresql://{POSTGRES_HOST}/{DATABASE_NAME}` |

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

| Variable    | Meaning                  |
|-------------|--------------------------|
| `.Host`     | Database host            |
| `.Role`     | Generated user/role name |
| `.Database` | Referenced database name |
| `.Password` | Generated role password  |

### Compatibility

Postgres operator uses Operator SDK, which uses kubernetes client. Kubernetes client compatibility with Kubernetes cluster
can be found [here](https://github.com/kubernetes/client-go/blob/master/README.md#compatibility-matrix)

Postgres operator compatibility with Operator SDK version is in the table below

|                           | Operator SDK version | apiextensions.k8s.io |
|---------------------------|----------------------|----------------------|
| `postgres-operator 0.4.x` | v0.17                |  v1beta1             |
| `postgres-operator 1.x.x` | v0.18                |  v1                  |
| `postgres-operator 2.x.x` | v1.39                |  v1                  |
| `HEAD`                    | v1.39                |  v1                  |


## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md)

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
