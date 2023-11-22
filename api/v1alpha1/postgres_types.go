/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// PostgresSpec defines the desired state of Postgres
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
// +kubebuilder:pruning:PreserveUnknownFields
type PostgresStatus struct {
	Succeeded bool `json:"succeeded"`
	// name of the created db
	DbName string        `json:"dbname"`
	Roles  PostgresRoles `json:"roles"`
	// +optional
	// +listType=set
	Schemas []string `json:"schemas,omitempty"`
	// +optional
	// +listType=set
	Extensions []string `json:"extensions,omitempty"`
}

// PostgresRoles stores the different group roles for database
type PostgresRoles struct {
	Owner  string `json:"owner"`
	Reader string `json:"reader"`
	Writer string `json:"writer"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced

// Postgres is the Schema for the postgres API
type Postgres struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PostgresSpec   `json:"spec,omitempty"`
	Status PostgresStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// PostgresList contains a list of Postgres
type PostgresList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Postgres `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Postgres{}, &PostgresList{})
}
