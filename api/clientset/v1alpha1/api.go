package v1alpha1

type ManagedNodeV1Alpha1Interface interface {
	ManagedNodes(namespace string) ManagedNodeInterface
}
