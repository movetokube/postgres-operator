package postgresuser

import (
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	dbv1alpha1 "github.com/movetokube/postgres-operator/pkg/apis/db/v1alpha1"
)

func TestNewSecretForCR_UserDefinedLabels(t *testing.T) {
	r := &ReconcilePostgresUser{
		pgHost:         "localhost",
		keepSecretName: false,
	}
	cr := &dbv1alpha1.PostgresUser{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myuser",
			Namespace: "myns",
		},
		Spec: dbv1alpha1.PostgresUserSpec{
			SecretName: "mysecret",
			Labels: map[string]string{
				"custom": "label",
				"foo":    "bar",
			},
			Annotations: map[string]string{
				"anno": "value",
			},
		},
		Status: dbv1alpha1.PostgresUserStatus{
			DatabaseName: "somedb",
		},
	}
	secret, err := r.newSecretForCR(cr, "role1", "pass1", "login1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedLabels := map[string]string{
		"app":    "myuser",
		"custom": "label",
		"foo":    "bar",
	}
	if !reflect.DeepEqual(secret.Labels, expectedLabels) {
		t.Errorf("labels mismatch: got %v, want %v", secret.Labels, expectedLabels)
	}
	if secret.Annotations["anno"] != "value" {
		t.Errorf("annotations mismatch: got %v", secret.Annotations)
	}
	expectedName := "mysecret-myuser"
	if secret.Name != expectedName {
		t.Errorf("secret name mismatch: got %s, want %s", secret.Name, expectedName)
	}
	if secret.Namespace != "myns" {
		t.Errorf("secret namespace mismatch: got %s", secret.Namespace)
	}
	if string(secret.Data["ROLE"]) != "role1" {
		t.Errorf("secret data ROLE mismatch: got %s", secret.Data["ROLE"])
	}
	if string(secret.Data["PASSWORD"]) != "pass1" {
		t.Errorf("secret data PASSWORD mismatch: got %s", secret.Data["PASSWORD"])
	}
	if string(secret.Data["LOGIN"]) != "login1" {
		t.Errorf("secret data LOGIN mismatch: got %s", secret.Data["LOGIN"])
	}
	if string(secret.Data["DATABASE_NAME"]) != "somedb" {
		t.Errorf("secret data DATABASE_NAME mismatch: got %s", secret.Data["DATABASE_NAME"])
	}
	if string(secret.Data["HOST"]) != "localhost" {
		t.Errorf("secret data HOST mismatch: got %s", secret.Data["HOST"])
	}
}

func TestNewSecretForCR_EmptyLabels(t *testing.T) {
	r := &ReconcilePostgresUser{
		pgHost:         "localhost",
		keepSecretName: false,
	}
	cr := &dbv1alpha1.PostgresUser{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myuser2",
			Namespace: "myns2",
		},
		Spec: dbv1alpha1.PostgresUserSpec{
			SecretName: "mysecret2",
			Labels:     map[string]string{},
		},
		Status: dbv1alpha1.PostgresUserStatus{
			DatabaseName: "somedb2",
		},
	}
	secret, err := r.newSecretForCR(cr, "role2", "pass2", "login2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedLabels := map[string]string{
		"app": "myuser2",
	}
	if !reflect.DeepEqual(secret.Labels, expectedLabels) {
		t.Errorf("labels mismatch: got %v, want %v", secret.Labels, expectedLabels)
	}
	expectedName := "mysecret2-myuser2"
	if secret.Name != expectedName {
		t.Errorf("secret name mismatch: got %s, want %s", secret.Name, expectedName)
	}
	if secret.Namespace != "myns2" {
		t.Errorf("secret namespace mismatch: got %s", secret.Namespace)
	}
}

func TestNewSecretForCR_KeepSecretName(t *testing.T) {
	r := &ReconcilePostgresUser{
		pgHost:         "localhost",
		keepSecretName: true,
	}
	cr := &dbv1alpha1.PostgresUser{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myuser3",
			Namespace: "myns3",
		},
		Spec: dbv1alpha1.PostgresUserSpec{
			SecretName: "mysecret3",
			Labels:     map[string]string{},
		},
		Status: dbv1alpha1.PostgresUserStatus{
			DatabaseName: "somedb3",
		},
	}
	secret, err := r.newSecretForCR(cr, "role3", "pass3", "login3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedName := "mysecret3"
	if secret.Name != expectedName {
		t.Errorf("secret name mismatch with keepSecretName: got %s, want %s", secret.Name, expectedName)
	}
}
