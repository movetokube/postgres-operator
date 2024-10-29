package postgresuser

import (
	"reflect"
	"testing"

	dbv1alpha1 "github.com/movetokube/postgres-operator/pkg/apis/db/v1alpha1"
)

func assertEqual(t *testing.T, a interface{}, b interface{}) {
	t.Helper()

	if a == b {
		return
	}

	// debug.PrintStack()
	t.Errorf("Received %v (type %v), expected %v (type %v)", a, reflect.TypeOf(a), b, reflect.TypeOf(b))
}

func TestReconcilePostgresUser_newSecretForCR(t *testing.T) {
	rpu := &ReconcilePostgresUser{
		pgHost: "localhost",
		pgPort: 5432,
	}

	cr := &dbv1alpha1.PostgresUser{
		Status: dbv1alpha1.PostgresUserStatus{
			DatabaseName: "dbname",
		},
		Spec: dbv1alpha1.PostgresUserSpec{
			SecretTemplate: map[string]string{
				"all": "host={{.Host}} host_no_port={{.HostNoPort}} port={{.Port}} user={{.Role}} login={{.Login}} password={{.Password}} dbname={{.Database}}",
			},
		},
	}

	secret, err := rpu.newSecretForCR(cr, "role", "password", "login")
	if err != nil {
		t.Fatalf("could not patch object: (%v)", err)
	}

	if secret == nil {
		t.Fatalf("no secret returned")
	}

	// keep old behavior of merging host and port
	assertEqual(t, string(secret.Data["HOST"]), "localhost:5432")
	assertEqual(t, string(secret.Data["all"]), "host=localhost:5432 host_no_port=localhost port=5432 user=role login=login password=password dbname=dbname")
}
