package v1alpha1

import (
	apiextensionv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//go:generate controller-gen object paths=$GOFILE

const (
	StatusManagedNodeNeedToCreated  = 0
	StatusManagedNodeCreation       = 1
	StatusManagedNodeCreated        = 2
	StatusManagedNodeCreationFailed = 3
	StatusManagedNodeDeletion       = 4
	StatusManagedNodeDeleted        = 5
	StatusManagedNodeGroupNotFound  = 6
)

type ManagedNodeStatus struct {
	apiextensionv1.CustomResourceSubresourceStatus `json:",inline"`
	// A human-readable description of the status of this operation.
	// +optional
	Message string `json:"message,omitempty" protobuf:"bytes,3,opt,name=message"`
	// A machine-readable description of why this operation is in the
	// "Failure" status. If this value is empty there
	// is no information available. A Reason clarifies an HTTP status
	// code but does not override it.
	// +optional
	Reason metav1.StatusReason `json:"reason,omitempty" protobuf:"bytes,4,opt,name=reason,casttype=StatusReason"`
	// Suggested HTTP return code for this status, 0 if not set.
	// +optional
	Code int32 `json:"code,omitempty" protobuf:"varint,6,opt,name=code"`
}

type ManagedNodeSpec struct {
	NodeGroup  string `json:"nodegroup,omitempty"`
	NodeName   string `json:"nodename,omitempty"`
	VCpus      int    `default:"2" json:"vcpus"`
	MemorySize int    `default:"2048" json:"memorySizeInMb"`
	DiskSize   int    `default:"10240" json:"diskSizeInMb"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ManagedNode struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ManagedNodeSpec   `json:"spec,omitempty"`
	Status            ManagedNodeStatus `json:"status,omitempty"`
}

var phStatusManagedNodeReason = []metav1.StatusReason{
	"StatusManagedNodeNeedToCreated",
	"StatusManagedNodeCreation",
	"StatusManagedNodeCreated",
	"StatusManagedNodeCreationFailed",
	"StatusManagedNodeDeletion",
	"StatusManagedNodeDeleted",
	"StatusManagedNodeGroupNotFound",
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ManagedNodeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []ManagedNode `json:"items"`
}

func StatusManagedNodeReason(code int) metav1.StatusReason {
	if code >= StatusManagedNodeNeedToCreated && code <= StatusManagedNodeGroupNotFound {
		return phStatusManagedNodeReason[code]
	}

	return "StatusManagedNodeUndefined"
}
