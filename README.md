# External PostgreSQL server operator for Kubernetes

## Features

* Creates a database from a CR
* Creates a role with random username and password from a CR
* If the database exist, it will only create a role
* Multiple user roles can own one database
* Creates Kubernetes secret with postgres_uri in the same namespace as CR
* Support for AWS RDS and Azure Database for PostgresSQL

## Cloud specific configuration

### AWS

In order for this operator to work correctly with AWS RDS, you need to set `POSTGRES_CLOUD_PROVIDER` to `AWS` either in
the ext-postgres-operator kubernetes secret or directly in the deployment manifest (`operator.yaml`).

### Azure Database for PostgreSQL

In order for this operator to work correctly with Azure managed PostgreSQL database, two env variables needs to be provided for the operator:

* `POSTGRES_CLOUD_PROVIDER` set to `Azure`
* `POSTGRES_DEFAULT_DATABASE` set to your default database, i.e. `postgres`

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

To install the operator, follow the steps below.

1. Configure Postgres credentials for the operator in `deploy/secret.yaml`
2. Create the operator with either\
    `kubectl kustomize deploy/ | apply -f -`\
    or by using [kustomize](https://github.com/kubernetes-sigs/kustomize) directly\
    `kustomize build deploy/ | apply -f -`
3. Apply the secret with\
   `kubectl apply -f deploy/secret.yaml`

## CRs

### Postgres

```yaml
apiVersion: db.movetokube.com/v1alpha1
kind: Postgres
metadata:
  name: my-db
  namespace: app
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
spec:
  role: username
  database: my-db # This references the Postgres CR
  secretName: my-secret
  privileges: OWNER # Can be OWNER/READ/WRITE
```

This creates a user role `username-<hash>` and grants role `test-db-group`, `test-db-writer` or `test-db-reader` depending on `privileges` property. Its credentials are put in secret `my-secret-my-db-user`.

`PostgresUser` needs to reference a `Postgres` in the same namespace.

Two `Postgres` referencing the same database can exist in more than one namespace. The last CR referencing a database will drop the group role and transfer database ownership to the role used by the operator.

### Contribution

You can contribute to this project by opening a PR to merge to `master`, or one of the `vX.X.X` branches.

#### Branching

`master` branch contains the latest source code with all the features. `vX.X.X` contains code for the specific major versions.
 i.e. `v0.4.x` contains the latest code for 0.4 version of the operator. See compatibility matrix below.

#### Tests

Please write tests and fix any broken tests before you open a PR. Tests should cover at least 80% of your code.

### Compatibility

Postgres operator uses Operator SDK, which uses kubernetes client. Kubernetes client compatibility with Kubernetes cluster
can be found [here](https://github.com/kubernetes/client-go/blob/master/README.md#compatibility-matrix)

Postgres operator compatibility with Operator SDK version is in the table below

|                               | Operator SDK 0.17.x | Operator SDK 1.9.0 |
|-------------------------------|---------------------|--------------------|
| `postgres-operator 0.4.x`     | ✓                   | -                  |
| `postgres-operator 1.0.x`     | -                   | ✓                  |
| `HEAD`                        | ✓                   | -                  |
