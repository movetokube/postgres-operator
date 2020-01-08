package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// PostgresSpec defines the desired state of Postgres
// +k8s:openapi-gen=true
type PostgresSpec struct {
	Database string `json:"database"`
	// +optional
	MasterRole string `json:"masterRole,omitempty"`
	// +optional
	DropOnDelete bool `json:"dropOnDelete,omitempty"`
	// +optional
	// +listType=set
	Schemas []string `json:"schemas,omitempty"`
	// +optional
	// +listType=set
	Extensions []string `json:"extensions,omitempty"`
}

// PostgresStatus defines the observed state of Postgres
// +k8s:openapi-gen=true
type PostgresStatus struct {
	Succeeded bool          `json:"succeeded"`
	Roles     PostgresRoles `json:"roles"`
	// +optional
	// +listType=set
	Schemas []string `json:"schemas,omitempty"`
	// +optional
	// +listType=set
	Extensions []string `json:"extensions,omitempty"`
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html
}

// PostgresRoles stores the different group roles for database
// +k8s:openapi-gen=true
type PostgresRoles struct {
	Owner  string `json:"owner"`
	Reader string `json:"reader"`
	Writer string `json:"writer"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Postgres is the Schema for the postgres API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
type Postgres struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PostgresSpec   `json:"spec,omitempty"`
	Status PostgresStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PostgresList contains a list of Postgres
type PostgresList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Postgres `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Postgres{}, &PostgresList{})
}
