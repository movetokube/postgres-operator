# External PostgreSQL server operator for Kubernetes

Starting with ext-postgres-operator Helm chart version **1.2.3** images will be pulled from ghcr by default, you can change this if you like.

Here's how to install it (please install with care according to your configuration):

```sh
helm repo add ext-postgres-operator https://tom-ha.github.io/postgres-operator/
helm upgrade --install -n operators ext-postgres-operator ext-postgres-operator/ext-postgres-operator
```

## Features

* Creates a database from a CR
* Creates a role with random username and password from a CR
* If the database exist, it will only create a role
* Multiple user roles can own one database
* Creates Kubernetes secret with postgres_uri in the same namespace as CR
* Support for AWS RDS and Azure Database for PostgresSQL
* Support for managing CRs in dynamically created namespaces

## Cloud specific configuration

### AWS

In order for this operator to work correctly with AWS RDS, you need to set `POSTGRES_CLOUD_PROVIDER` to `AWS` either in
the ext-postgres-operator kubernetes secret or directly in the deployment manifest (`operator.yaml`).

### Azure Database for PostgreSQL (both Single Server and Flexible Server)

In order for this operator to work correctly with Azure managed PostgreSQL database, two env variables needs to be provided for the operator:

* `POSTGRES_CLOUD_PROVIDER` set to `Azure`
* `POSTGRES_DEFAULT_DATABASE` set to your default database, i.e. `postgres`

### GCP

In order for this operator to work correctly with GCP, you need to set `POSTGRES_CLOUD_PROVIDER` to `GCP` 

To have operator work with GCP properly you have to:

* use postgresql connection in secret
* manually create a Master role e.g. "devops-operators"
* use such role in database CR e.g. spec.masterRole: devops-operator

DropRole method will check for db owner and will skip master role dropping

## General Configuration

These environment variables are embedded in [deploy/operator.yaml](deploy/operator.yaml), `env` section.

* `WATCH_NAMESPACE` - which namespace to watch. Defaults to empty string for all namespaces
* `OPERATOR_NAME` - name of the operator, defaults to `ext-postgres-operator` 
* `POSTGRES_INSTANCE` - identity of operator, this matched with `postgres.db.movetokube.com/instance` in CRs. Default is empty

`POSTGRES_INSTANCE` is only available since version 1.2.0

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

1. Configure Postgres credentials for the operator in `deploy/secret.yaml`
2. Create namespace if needed with\
   `kubectl apply -f deploy/namespace.yaml`
3. Apply the secret with\
   `kubectl apply -f deploy/secret.yaml`
4. Create the operator with either\
    `kubectl kustomize deploy/ | apply -f -`\
    or by using [kustomize](https://github.com/kubernetes-sigs/kustomize) directly\
    `kustomize build deploy/ | apply -f -`

Alternatively you can install operator using Helm Chart located in the
`charts/ext-postgres-operator` subdirectory. Sample installation commands provided below:

```sh
helm repo add ext-postgres-operator https://tom-ha.github.io/postgres-operator/
helm install -n operators ext-postgres-operator  ext-postgres-operator/ext-postgres-operator
```

See [values.yaml](charts/ext-postgres-operator/values.yaml) for the possible values to define.

## CRs

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
```

This creates a user role `username-<hash>` and grants role `test-db-group`, `test-db-writer` or `test-db-reader` depending on `privileges` property. Its credentials are put in secret `my-secret-my-db-user`.

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

Since version 1.2 it is possible to use many instances of postgres-operator to control different databases based on annotations in CRs.
Follow the steps below to enable multi-operator support.

1. Add POSTGRES_INSTANCE

#### Annotations Use Case

With the help of annotations it is possible to create annotation-based copies of secrets in other namespaces.

For more information and an example, see [kubernetes-replicator#pull-based-replication](https://github.com/mittwald/kubernetes-replicator#pull-based-replication)

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

|                           | Operator SDK version | apiextensions.k8s.io |
|---------------------------|----------------------|----------------------|
| `postgres-operator 0.4.x` | v0.17                |  v1beta1             |
| `postgres-operator 1.x.x` | v0.18                |  v1                  |
| `HEAD`                    | v0.18                |  v1                  |
