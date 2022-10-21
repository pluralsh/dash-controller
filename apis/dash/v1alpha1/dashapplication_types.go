package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

func init() {
	SchemeBuilder.Register(&DashApplication{}, &DashApplicationList{})
}

type DashApplicationSpec struct {
	// Image name
	Image string `json:"image"`
	// ContainerPort port number for image container
	ContainerPort int32 `json:"containerPort"`

	// Number of desired pods. This is a pointer to distinguish between explicit
	// zero and not specified. Defaults to 1.
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`
	// +optional
	// Labels for dash deployment
	Labels map[string]string `json:"labels,omitempty"`
	// +optional
	// ServiceAnnotations for dash k8s service
	ServiceAnnotations map[string]string `json:"serviceAnnotations,omitempty"`
}

type DashApplicationStatus struct {
	// Ready is true when the provider resource is ready.
	// +optional
	Ready bool `json:"ready"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="Application ready status"
type DashApplication struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DashApplicationSpec   `json:"spec,omitempty"`
	Status DashApplicationStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type DashApplicationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DashApplication `json:"items"`
}
