# External PostgreSQL server operator for Kubernetes

## Features
* creates a database
* creates a role with random username and password
* assigns role to the database
* if the database exist, it will only create a role
* drops role when removing CR, assigns all objects to user `postgres`
* creates a Kubernetes secret with postgres_uri in the same namespace as CR

CR example
```yaml
apiVersion: db.movetokube.com/v1alpha1
kind: Postgres
metadata:
  name: my-db
  namespace: app
spec:
  # Add fields here
  database: test-db
  secretName: my-secret
```

## Installation

1. Configure Postgres credentials for the operator in `deploy/operator.yaml` 
2. `kubectl apply -f deploy/crds/db_v1alpha1_postgres_crd.yaml`
3. `kubectl apply -f deploy/namespace.yaml`
4. `kubectl apply -f role.yaml`
5. `kubectl apply -f role_binding.yaml`
6. `kubectl apply -f service_account.yaml`
7. `kubectl apply -f operator.yaml`

