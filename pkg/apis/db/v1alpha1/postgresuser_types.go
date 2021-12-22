package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// PostgresUserSpec defines the desired state of PostgresUser
// +k8s:openapi-gen=true
type PostgresUserSpec struct {
	Role       string `json:"role"`
	Database   string `json:"database"`
	SecretName string `json:"secretName"`
	// +optional
	Privileges string `json:"privileges"`
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// PostgresUserStatus defines the observed state of PostgresUser
// +k8s:openapi-gen=true
type PostgresUserStatus struct {
	Succeeded     bool   `json:"succeeded"`
	PostgresRole  string `json:"postgresRole"`
	PostgresLogin string `json:"postgresLogin"`
	PostgresGroup string `json:"postgresGroup"`
	DatabaseName  string `json:"databaseName"`
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PostgresUser is the Schema for the postgresusers API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
type PostgresUser struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PostgresUserSpec   `json:"spec,omitempty"`
	Status PostgresUserStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PostgresUserList contains a list of PostgresUser
type PostgresUserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PostgresUser `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PostgresUser{}, &PostgresUserList{})
}
