package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// CanaryGateSpec defines the desired state of CanaryGate
type CanaryGateSpec struct {
	ConfirmRollout         string `json:"confirm-rollout,omitempty"`
	PreRollout             string `json:"pre-rollout,omitempty"`
	Rollout                string `json:"rollout,omitempty"`
	ConfirmTrafficIncrease string `json:"confirm-traffic-increase,omitempty"`
	ConfirmPromotion       string `json:"confirm-promotion,omitempty"`
	PostRollout            string `json:"post-rollout,omitempty"`
	Rollback               string `json:"rollback,omitempty"`
	Target                 Target `json:"target,omitempty"`

	// Flagger contains the raw spec for the Flagger Canary resource.
	// We use RawExtension to capture all fields dynamically.
	// +kubebuilder:pruning:PreserveUnknownFields
	Flagger runtime.RawExtension `json:"flagger"`
}

// Target defines target Flagger Canary resource
type Target struct {
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
}

// CanaryGateStatus defines the observed state of CanaryGate
type CanaryGateStatus struct {
	// Name of the canary
	Name string `json:"name"`
	// Namespace of the canary
	Namespace string `json:"namespace"`
	// Gate status
	Status string `json:"status"`
	// Gate Message
	Message string `json:"message,omitempty"`
	// Gate Target (Name and Namespace)
	Target string `json:"target,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// CanaryGate is the Schema for the canarygates API
type CanaryGate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CanaryGateSpec   `json:"spec,omitempty"`
	Status CanaryGateStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// CanaryGateList contains a list of CanaryGate
type CanaryGateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CanaryGate `json:"items"`
}

func init() {
	// Run `controller-gen object paths=./api/v1beta1/..` to get the generated code
	SchemeBuilder.Register(&CanaryGate{}, &CanaryGateList{})
}
