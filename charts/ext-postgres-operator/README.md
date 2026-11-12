# ext-postgres-operator Helm Chart

This Helm chart deploys the External Postgres Operator, which provides a way to manage PostgreSQL databases and users in a Kubernetes environment.

## Installation

To install the chart, add the repository and use the `helm upgrade --install` command:

```bash
helm repo add ext-postgres-operator https://movetokube.github.io/postgres-operator/
helm upgrade --install -n operators ext-postgres-operator ext-postgres-operator/ext-postgres-operator
```

## Compatibility

**NOTE:** Helm chart version `>= 3.0.0` requires External Secret Operator version `>= 0.17.0`. Ensure that you are using the correct versions to avoid compatibility issues.

**NOTE:** Helm chart version `>= 2.0.0` is only compatible with the Postgres Operator version `2.0.0`. Ensure that you are using the correct versions to avoid compatibility issues.

## Namespaced RBAC

By default a ClusterRole grants the operator Secrets and its custom resources in every namespace. To confine an instance to its own namespace, set `rbac.clusterWide: false`:

- no ClusterRole or ClusterRoleBinding is rendered;
- the operator watches only the release namespace (`watchNamespace` must be empty or the release namespace);
- its Role grants only its custom resources (with `/status` and `/finalizers`), events, its leader election lease, and Secrets without `list` or `watch`.

`rbac.secretNames` further limits `get`, `update`, `patch` and `delete` on Secrets to the Secrets its `PostgresUser`s write (`create` can't be limited by name):

```yaml
rbac:
  clusterWide: false
  secretNames:
    - my-app-db
env:
  KEEP_SECRET_NAME: "true"
```

Without `KEEP_SECRET_NAME`, a `PostgresUser`'s Secret is named `<secretName>-<PostgresUser name>`. If `POSTGRES_INSTANCE` is set, set it through `env`, so the Role names the right lease.

Namespaced RBAC needs an operator version that reads Secrets without the informer cache.
