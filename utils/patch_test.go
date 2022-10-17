package utils

import (
	"context"
	"strconv"
	"testing"

	"github.com/movetokube/postgres-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	name      string = "test-db"
	namespace string = "operator"
)

var postgres *v1alpha1.Postgres = &v1alpha1.Postgres{
	ObjectMeta: metav1.ObjectMeta{
		Name:      name,
		Namespace: namespace,
	},
	Spec: v1alpha1.PostgresSpec{
		Database: name,
	},
}
var objs []runtime.Object = []runtime.Object{postgres}

func TestPatchUtilShouldPatchIfThereIsDifference(t *testing.T) {
	// Create modified postgres
	modPostgres := postgres.DeepCopy()
	modPostgres.Spec.DropOnDelete = true
	modPostgres.Status.Succeeded = true

	// Create runtime scheme
	s := scheme.Scheme
	s.AddKnownTypes(v1alpha1.GroupVersion, &v1alpha1.Postgres{})

	// Create fake client to mock API calls
	cl := fake.NewFakeClient(objs...)

	// Patch object
	err := Patch(cl, context.TODO(), postgres, modPostgres)
	if err != nil {
		t.Fatalf("could not patch object: (%v)", err)
	}

	// Check if postgres is identical to modified object
	foundPostgres := &v1alpha1.Postgres{}
	err = cl.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, foundPostgres)
	if err != nil {
		t.Fatalf("could not get postgres: (%v)", err)
	}
	// Comparison
	if foundPostgres.Spec.DropOnDelete != modPostgres.Spec.DropOnDelete {
		t.Fatalf("found Postgres is not identical to modified Postgres: DropOnDelete == %s, expected %s", strconv.FormatBool(foundPostgres.Spec.DropOnDelete), strconv.FormatBool(modPostgres.Spec.DropOnDelete))
	}
	if foundPostgres.Status.Succeeded != modPostgres.Status.Succeeded {
		t.Fatalf("found Postgres is not identical to modified Postgres: Succeeded == %s, expected %s", strconv.FormatBool(foundPostgres.Status.Succeeded), strconv.FormatBool(modPostgres.Status.Succeeded))
	}
}
