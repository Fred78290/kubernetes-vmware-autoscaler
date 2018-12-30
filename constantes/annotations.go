package constantes

const (
	// NodeLabelGroupName k8s annotation
	NodeLabelGroupName = "cluster.autoscaler.nodegroup/name"

	// AnnotationNodeIndex k8s annotation
	AnnotationNodeIndex = "cluster.autoscaler.nodegroup/node-index"

	// AnnotationNodeAutoProvisionned k8s annotation
	AnnotationNodeAutoProvisionned = "cluster.autoscaler.nodegroup/autoprovision"

	// AnnotationScaleDownDisabled k8s annotation
	AnnotationScaleDownDisabled = "cluster-autoscaler.kubernetes.io/scale-down-disabled"
)
