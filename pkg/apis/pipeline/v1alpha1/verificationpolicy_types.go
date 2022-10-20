/*
Copyright 2022 The Tekton Authors
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
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// +genclient
// +genclient:noStatus
// +genreconciler:krshapedlogic=false
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VerificationPolicy
//
// +k8s:openapi-gen=true
type VerificationPolicy struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata"`

// Spec holds the desired state of the VerificationPolicy.
	Spec VerificationPolicySpec `json:"spec"`
}

// VerificationPolicyList contains a list of Run
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type VerificationPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VerificationPolicy `json:"items"`
}

// GetGroupVersionKind implements kmeta.OwnerRefable.
func (*VerificationPolicy) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(pipeline.VerificationPolicyName)
}

type VerificationPolicySpec struct {
	// Resources defines the patterns of Resources names that should be subject to this policy. For example, we may want to apply this Policy only from a certain github repo. Then the ResourcesPattern should include the path. If using gitresolver, and we want to config keys from a certain git repo. `ResourcesPattern` can be `https://github.com/tektoncd/catalog.git`, we will use regex to filter out those resources.
	//Resources []ResourcePattern `json:"resources"`
	ResourcesPolicyMapping map[ResourcePattern][]Authority `json:"resourcespolicymapping"`
}

type ResourcePattern struct {
	// Pattern defines a resource pattern. Regex is created to filter resources based on `Pattern`
	Pattern string `json:"pattern"`
}

// The authorities block defines the rules for discovering and
// validating signatures.
type Authority struct {
	// Name is the name for this authority.
	Name string `json:"name"`
	// Key defines the type of key to validate the resource.
	// +optional
	Key *KeyRef `json:"key,omitempty"`
}

type KeyRef struct {
	// SecretRef sets a reference to a secret with the key.
	// +optional
	SecretRef *v1.SecretReference `json:"secretRef,omitempty"`
	// Data contains the inline public key.
	// +optional
	Data string `json:"data,omitempty"`
	// KMS contains the KMS url of the public key
	// Supported formats differ based on the KMS system used.
	// +optional
	KMS string `json:"kms,omitempty"`
	// HashAlgorithm always defaults to sha256 if the algorithm hasn't been explicitly set
	// +optional
	HashAlgorithm string `json:"hashAlgorithm,omitempty"`
}
