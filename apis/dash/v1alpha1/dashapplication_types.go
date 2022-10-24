package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

func init() {
	SchemeBuilder.Register(&DashApplication{}, &DashApplicationList{})
}

type DashApplicationSpec struct {
	// Container spec
	Container Container `json:"container"`
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
	// Ingress spec. If not specified only LoadBalancer service is created
	// +optional
	Ingress *Ingress `json:"ingress,omitempty"`
}

// A single application container that you want to run.
type Container struct {
	// Image name
	Image string `json:"image"`
	// Entrypoint array. Not executed within a shell.
	// +optional
	Command []string `json:"command,omitempty"`
	// Arguments to the entrypoint.
	// +optional
	Args []string `json:"args,omitempty"`
	// ContainerPort port number for image container
	ContainerPort int32 `json:"containerPort"`
}

type Ingress struct {
	// Host is the fully qualified domain name of a network host, as defined by RFC 3986.
	// Note the following deviations from the "host" part of the
	// URI as defined in RFC 3986:
	// 1. IPs are not allowed. Currently an IngressRuleValue can only apply to
	//    the IP in the Spec of the parent Ingress.
	// 2. The `:` delimiter is not respected because ports are not allowed.
	//	  Currently the port of an Ingress is implicitly :80 for http and
	//	  :443 for https.
	// Both these may change in the future.
	// Incoming requests are matched against the host before the
	// IngressRuleValue. If the host is unspecified, the Ingress routes all
	// traffic based on the specified IngressRuleValue.
	//
	// Host can be "precise" which is a domain name without the terminating dot of
	// a network host (e.g. "foo.bar.com") or "wildcard", which is a domain name
	// prefixed with a single wildcard label (e.g. "*.foo.com").
	// The wildcard character '*' must appear by itself as the first DNS label and
	// matches only a single label. You cannot have a wildcard label by itself (e.g. Host == "*").
	// Requests will be matched against the Host field in the following way:
	// 1. If Host is precise, the request matches this rule if the http host header is equal to Host.
	// 2. If Host is a wildcard, then the request matches this rule if the http host header
	// is to equal to the suffix (removing the first label) of the wildcard rule.
	// +optional
	Host string `json:"host,omitempty"`
	// Path is matched against the path of an incoming request. Currently it can
	// contain characters disallowed from the conventional "path" part of a URL
	// as defined by RFC 3986. Paths must begin with a '/' and must be present
	// when using PathType with value "Exact" or "Prefix".
	// +optional
	Path string `json:"path,omitempty"`
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
