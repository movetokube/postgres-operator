/*
Copyright 2022.

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

// PostgresUserSpec defines the desired state of PostgresUser
type PostgresUserSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Role       string `json:"role"`
	Database   string `json:"database"`
	SecretName string `json:"secretName"`
	// +optional
	Privileges string `json:"privileges"`
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// PostgresUserStatus defines the observed state of PostgresUser
type PostgresUserStatus struct {
	Succeeded     bool   `json:"succeeded"`
	PostgresRole  string `json:"postgresRole"`
	PostgresLogin string `json:"postgresLogin"`
	PostgresGroup string `json:"postgresGroup"`
	DatabaseName  string `json:"databaseName"`
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

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

//+kubebuilder:object:root=true

// PostgresUserList contains a list of PostgresUser
type PostgresUserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PostgresUser `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PostgresUser{}, &PostgresUserList{})
}
