package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const GroupName = "nodemanager.aldunelabs.com"
const GroupVersion = "v1alpha1"
const CRDSingular = "managednode"
const CRDPlural = "managednodes"
const CRDShortName = "mn"
const FullCRDName = CRDPlural + "." + GroupName

var SchemeGroupVersion = schema.GroupVersion{
	Group:   GroupName,
	Version: GroupVersion,
}

var (
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	AddToScheme   = SchemeBuilder.AddToScheme
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&ManagedNode{},
		&ManagedNodeList{},
	)

	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)

	return nil
}
