package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// PostgresUserSpec defines the desired state of PostgresUser
type PostgresUserSpec struct {
	Role       string `json:"role"`
	Database   string `json:"database"`
	SecretName string `json:"secretName"`
	// +optional
	SecretTemplate map[string]string `json:"secretTemplate,omitempty"` // key-value, where key is secret field, value is go template
	// +optional
	Privileges string `json:"privileges"`
	// +optional
	AWS *PostgresUserAWSSpec `json:"aws,omitempty"`
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
}

// PostgresUserAWSSpec encapsulates AWS specific configuration toggles.
type PostgresUserAWSSpec struct {
	// +optional
	EnableIamAuth bool `json:"enableIamAuth,omitempty"`
}

// PostgresUserStatus defines the observed state of PostgresUser
type PostgresUserStatus struct {
	Succeeded     bool   `json:"succeeded"`
	PostgresRole  string `json:"postgresRole"`
	PostgresLogin string `json:"postgresLogin"`
	PostgresGroup string `json:"postgresGroup"`
	DatabaseName  string `json:"databaseName"`
	EnableIamAuth bool   `json:"enableIamAuth"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced

// PostgresUser is the Schema for the postgresusers API
type PostgresUser struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PostgresUserSpec   `json:"spec,omitempty"`
	Status PostgresUserStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PostgresUserList contains a list of PostgresUser
type PostgresUserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PostgresUser `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PostgresUser{}, &PostgresUserList{})
}
