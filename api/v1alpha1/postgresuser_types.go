package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// PostgresUserSpec defines the desired state of PostgresUser
type PostgresUserSpec struct {
	Role string `json:"role"`
	// Deprecated: use Databases instead
	Database   string `json:"database,omitempty"`
	SecretName string `json:"secretName"`
	// +optional
	SecretTemplate map[string]string `json:"secretTemplate,omitempty"` // key-value, where key is secret field, value is go template
	// +optional
	// Deprecated: use Databases[].privileges instead
	Privileges string `json:"privileges,omitempty"`
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
	// +optional
	// +listType=map
	// +listMapKey=name
	Databases []PostgresUserDatabaseRef `json:"databases,omitempty"`
}

// PostgresUserDatabaseRef references a Postgres CR and desired privileges
type PostgresUserDatabaseRef struct {
	// name of the Postgres CR in the same namespace
	Name string `json:"name"`
	// Privileges: one of OWNER, WRITE, READ
	Privileges string `json:"privileges"`
}

// PostgresUserStatus defines the observed state of PostgresUser
type PostgresUserStatus struct {
	Succeeded     bool   `json:"succeeded"`
	PostgresRole  string `json:"postgresRole"`
	PostgresLogin string `json:"postgresLogin"`
	// Deprecated: for multi-db, use Grants
	PostgresGroup string `json:"postgresGroup,omitempty"`
	// Deprecated: for multi-db, use Grants
	DatabaseName string `json:"databaseName,omitempty"`
	// +optional
	// +listType=map
	// +listMapKey=databaseName
	Grants []PostgresUserDatabaseGrant `json:"grants,omitempty"`
}

// PostgresUserDatabaseGrant stores the granted group per database
type PostgresUserDatabaseGrant struct {
	DatabaseName  string `json:"databaseName"`
	PostgresGroup string `json:"postgresGroup"`
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
