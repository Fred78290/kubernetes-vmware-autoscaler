package v1alpha1

type ScalerNodeV1Alpha1Interface interface {
	ScalerNodes(namespace string) ScalerNodeInterface
}
