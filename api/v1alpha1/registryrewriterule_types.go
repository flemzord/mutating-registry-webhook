/*
Copyright 2025 flemzord.

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

// Rule defines a single registry rewrite rule
type Rule struct {
	// Match is a RE2 regular expression pattern to match against image names
	// +kubebuilder:validation:Required
	Match string `json:"match"`

	// Replace is a Go text/template string using captured groups from the match
	// +kubebuilder:validation:Required
	Replace string `json:"replace"`

	// Priority defines the order of rule evaluation (higher = more priority)
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=0
	Priority int `json:"priority,omitempty"`

	// Conditions specify when this rule should be applied
	// +kubebuilder:validation:Optional
	Conditions *RuleConditions `json:"conditions,omitempty"`
}

// RuleConditions defines conditions for when a rule should be applied
type RuleConditions struct {
	// Namespaces is a list of namespaces where this rule applies
	// +kubebuilder:validation:Optional
	Namespaces []string `json:"namespaces,omitempty"`

	// Labels is a map of labels that must match for the rule to apply
	// +kubebuilder:validation:Optional
	Labels map[string]string `json:"labels,omitempty"`
}

// RegistryRewriteRuleSpec defines the desired state of RegistryRewriteRule.
type RegistryRewriteRuleSpec struct {
	// Rules is a list of registry rewrite rules
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Rules []Rule `json:"rules"`
}

// RegistryRewriteRuleStatus defines the observed state of RegistryRewriteRule.
type RegistryRewriteRuleStatus struct {
	// ObservedGeneration is the generation observed by the controller
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Ready indicates if the rules are ready to be used
	Ready bool `json:"ready,omitempty"`

	// RuleCount is the number of rules in this resource
	RuleCount int `json:"ruleCount,omitempty"`

	// LastUpdateTime is the last time the rules were updated
	LastUpdateTime *metav1.Time `json:"lastUpdateTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=rrr
// +kubebuilder:printcolumn:name="Rules",type="integer",JSONPath=".status.ruleCount",description="Number of rules"
// +kubebuilder:printcolumn:name="Ready",type="boolean",JSONPath=".status.ready",description="Whether the rules are ready"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// RegistryRewriteRule is the Schema for the registryrewriterules API.
type RegistryRewriteRule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RegistryRewriteRuleSpec   `json:"spec,omitempty"`
	Status RegistryRewriteRuleStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RegistryRewriteRuleList contains a list of RegistryRewriteRule.
type RegistryRewriteRuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RegistryRewriteRule `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RegistryRewriteRule{}, &RegistryRewriteRuleList{})
}
