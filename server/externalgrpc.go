package server

import (
	"context"
	"fmt"

	"github.com/Fred78290/kubernetes-vmware-autoscaler/constantes"
	"github.com/Fred78290/kubernetes-vmware-autoscaler/externalgrpc"
	apigrpc "github.com/Fred78290/kubernetes-vmware-autoscaler/grpc"
	"github.com/Fred78290/kubernetes-vmware-autoscaler/types"
	"github.com/Fred78290/kubernetes-vmware-autoscaler/utils"
	glog "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/anypb"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type externalgrpcServerApp struct {
	externalgrpc.UnimplementedCloudProviderServer
	appServer        *AutoScalerServerApp
	autoProvisionned bool
}

func NewExternalgrpcServerApp(appServer *AutoScalerServerApp) (*externalgrpcServerApp, error) {
	external := &externalgrpcServerApp{
		appServer: appServer,
	}

	return external, external.doAutoProvision()
}

func (v *externalgrpcServerApp) doAutoProvision() error {
	if !v.autoProvisionned {

		nodesDefinition := make([]*apigrpc.NodeGroupDef, 0, len(v.appServer.configuration.VMwareInfos))

		for groupIdentifier := range v.appServer.configuration.VMwareInfos {
			nodegroupDef := &apigrpc.NodeGroupDef{
				NodeGroupID:         groupIdentifier,
				MinSize:             int32(v.appServer.configuration.MinNode),
				MaxSize:             int32(v.appServer.configuration.MaxNode),
				IncludeExistingNode: true,
				Labels:              v.appServer.configuration.NodeLabels,
				Provisionned:        true,
			}

			nodesDefinition = append(nodesDefinition, nodegroupDef)
		}

		v.appServer.NodesDefinition = nodesDefinition
		v.appServer.AutoProvision = true

		if err := v.appServer.doAutoProvision(); err != nil {
			glog.Errorf(constantes.ErrUnableToAutoProvisionNodeGroup, err)

			return err
		}

		v.autoProvisionned = true
	}

	return nil
}

// NodeGroups returns all node groups configured for this cloud provider.
func (v *externalgrpcServerApp) NodeGroups(ctx context.Context, request *externalgrpc.NodeGroupsRequest) (*externalgrpc.NodeGroupsResponse, error) {
	glog.Debugf("Call server NodeGroups: %v", request)

	nodeGroups := make([]*externalgrpc.NodeGroup, 0, len(v.appServer.Groups))

	for name, nodeGroup := range v.appServer.Groups {
		// Return node group if created
		if nodeGroup.Status == NodegroupCreated {
			nodeGroups = append(nodeGroups, &externalgrpc.NodeGroup{
				Id:      name,
				MinSize: int32(nodeGroup.MinNodeSize),
				MaxSize: int32(nodeGroup.MaxNodeSize),
			})
		}
	}

	return &externalgrpc.NodeGroupsResponse{
		NodeGroups: nodeGroups,
	}, nil
}

// NodeGroupForNode returns the node group for the given node.
// The node group id is an empty string if the node should not
// be processed by cluster autoscaler.
func (v *externalgrpcServerApp) NodeGroupForNode(ctx context.Context, request *externalgrpc.NodeGroupForNodeRequest) (*externalgrpc.NodeGroupForNodeResponse, error) {
	glog.Debugf("Call server NodeGroupForNode: %v", request)

	if nodegroupName, found := request.Node.Annotations[constantes.AnnotationNodeGroupName]; found {
		nodeGroup, err := v.appServer.getNodeGroup(nodegroupName)

		if err != nil {
			return nil, err
		}

		if nodeGroup == nil {
			glog.Infof("Nodegroup not found for node:%s", request.Node.Name)

			return nil, fmt.Errorf(constantes.ErrNodeGroupForNodeNotFound, nodegroupName, request.Node.Name)
		}

		return &externalgrpc.NodeGroupForNodeResponse{
			NodeGroup: &externalgrpc.NodeGroup{
				Id:      nodeGroup.NodeGroupIdentifier,
				MinSize: int32(nodeGroup.MinNodeSize),
				MaxSize: int32(nodeGroup.MaxNodeSize),
			},
		}, nil
	} else {
		return &externalgrpc.NodeGroupForNodeResponse{}, nil
	}
}

// PricingNodePrice returns a theoretical minimum price of running a node for
// a given period of time on a perfectly matching machine.
// Implementation optional.
func (v *externalgrpcServerApp) PricingNodePrice(ctx context.Context, request *externalgrpc.PricingNodePriceRequest) (*externalgrpc.PricingNodePriceResponse, error) {
	glog.Debugf("Call server NodePrice: %v", request)

	return &externalgrpc.PricingNodePriceResponse{
		Price: v.appServer.configuration.NodePrice,
	}, nil
}

// PricingPodPrice returns a theoretical minimum price of running a pod for a given
// period of time on a perfectly matching machine.
// Implementation optional.
func (v *externalgrpcServerApp) PricingPodPrice(ctx context.Context, request *externalgrpc.PricingPodPriceRequest) (*externalgrpc.PricingPodPriceResponse, error) {
	glog.Debugf("Call server PodPrice: %v", request)

	return &externalgrpc.PricingPodPriceResponse{
		Price: v.appServer.configuration.PodPrice,
	}, nil
}

// GPULabel returns the label added to nodes with GPU resource.
func (v *externalgrpcServerApp) GPULabel(ctx context.Context, request *externalgrpc.GPULabelRequest) (*externalgrpc.GPULabelResponse, error) {
	return &externalgrpc.GPULabelResponse{
		Label: "",
	}, nil
}

// GetAvailableGPUTypes return all available GPU types cloud provider supports.
func (v *externalgrpcServerApp) GetAvailableGPUTypes(ctx context.Context, request *externalgrpc.GetAvailableGPUTypesRequest) (*externalgrpc.GetAvailableGPUTypesResponse, error) {

	return &externalgrpc.GetAvailableGPUTypesResponse{
		GpuTypes: map[string]*anypb.Any{
			"nvidia-tesla-k80":  {},
			"nvidia-tesla-p100": {},
			"nvidia-tesla-v100": {},
		},
	}, nil
}

// Cleanup cleans up open resources before the cloud provider is destroyed, i.e. go routines etc.
func (v *externalgrpcServerApp) Cleanup(ctx context.Context, request *externalgrpc.CleanupRequest) (*externalgrpc.CleanupResponse, error) {
	glog.Debugf("Call server Cleanup: %v", request)

	var lastError error

	for _, nodeGroup := range v.appServer.Groups {
		if err := nodeGroup.cleanup(v.appServer.kubeClient); err != nil {
			lastError = err
		}
	}

	glog.Debug("Leave server Cleanup, done")

	v.appServer.Groups = make(map[string]*AutoScalerServerNodeGroup)

	return &externalgrpc.CleanupResponse{}, lastError
}

// Refresh is called before every main loop and can be used to dynamically update cloud provider state.
func (v *externalgrpcServerApp) Refresh(ctx context.Context, request *externalgrpc.RefreshRequest) (*externalgrpc.RefreshResponse, error) {
	for _, ng := range v.appServer.Groups {
		ng.refresh()
	}

	if phSaveState {
		if err := v.appServer.Save(phSavedState); err != nil {
			glog.Errorf(constantes.ErrFailedToSaveServerState, err)
		}
	}

	return &externalgrpc.RefreshResponse{}, nil
}

// NodeGroupTargetSize returns the current target size of the node group. It is possible
// that the number of nodes in Kubernetes is different at the moment but should be equal
// to the size of a node group once everything stabilizes (new nodes finish startup and
// registration or removed nodes are deleted completely).
func (v *externalgrpcServerApp) NodeGroupTargetSize(ctx context.Context, request *externalgrpc.NodeGroupTargetSizeRequest) (*externalgrpc.NodeGroupTargetSizeResponse, error) {
	glog.Debugf("Call server TargetSize: %v", request)

	nodeGroup := v.appServer.Groups[request.GetId()]

	if nodeGroup == nil {
		glog.Errorf(constantes.ErrNodeGroupNotFound, request.GetId())

		return nil, fmt.Errorf(constantes.ErrNodeGroupNotFound, request.GetId())
	}

	return &externalgrpc.NodeGroupTargetSizeResponse{
		TargetSize: int32(nodeGroup.targetSize()),
	}, nil
}

// NodeGroupIncreaseSize increases the size of the node group. To delete a node you need
// to explicitly name it and use NodeGroupDeleteNodes. This function should wait until
// node group size is updated.
func (v *externalgrpcServerApp) NodeGroupIncreaseSize(ctx context.Context, request *externalgrpc.NodeGroupIncreaseSizeRequest) (*externalgrpc.NodeGroupIncreaseSizeResponse, error) {
	glog.Debugf("Call server IncreaseSize: %v", request)

	nodeGroup := v.appServer.Groups[request.GetId()]

	if nodeGroup == nil {
		glog.Errorf(constantes.ErrNodeGroupNotFound, request.GetId())

		return nil, fmt.Errorf(constantes.ErrNodeGroupNotFound, request.GetId())
	}

	if request.GetDelta() <= 0 {
		glog.Errorf(constantes.ErrIncreaseSizeMustBePositive)

		return nil, fmt.Errorf(constantes.ErrIncreaseSizeMustBePositive)
	}

	newSize := nodeGroup.targetSize() + int(request.GetDelta())

	if newSize > nodeGroup.MaxNodeSize {
		glog.Errorf(constantes.ErrIncreaseSizeTooLarge, newSize, nodeGroup.MaxNodeSize)

		return nil, fmt.Errorf(constantes.ErrIncreaseSizeTooLarge, newSize, nodeGroup.MaxNodeSize)
	}

	go nodeGroup.setNodeGroupSize(v.appServer.kubeClient, newSize)

	return &externalgrpc.NodeGroupIncreaseSizeResponse{}, nil
}

// NodeGroupDeleteNodes deletes nodes from this node group (and also decreasing the size
// of the node group with that). Error is returned either on failure or if the given node
// doesn't belong to this node group. This function should wait until node group size is updated.
func (v *externalgrpcServerApp) NodeGroupDeleteNodes(ctx context.Context, request *externalgrpc.NodeGroupDeleteNodesRequest) (*externalgrpc.NodeGroupDeleteNodesResponse, error) {
	glog.Debugf("Call server DeleteNodes: %v", request)

	var err error

	nodeGroup := v.appServer.Groups[request.GetId()]

	if nodeGroup == nil {
		glog.Errorf(constantes.ErrNodeGroupNotFound, request.GetId())

		return nil, fmt.Errorf(constantes.ErrNodeGroupNotFound, request.GetId())
	}

	if nodeGroup.targetSize()-len(request.Nodes) < nodeGroup.MinNodeSize {
		return nil, fmt.Errorf(constantes.ErrMinSizeReached, request.GetId())
	}

	go func() {
		// Iterate over each requested node to delete
		for _, node := range request.Nodes {
			// Check node group owner
			if nodegroupName, found := node.Annotations[constantes.AnnotationNodeGroupName]; found {
				if nodeGroup, err = v.appServer.getNodeGroup(nodegroupName); err != nil {
					glog.Errorf(constantes.ErrNodeGroupNotFound, nodegroupName)
				}

				// Delete the node in the group
				if err = nodeGroup.deleteNodeByName(v.appServer.kubeClient, node.Name); err != nil {
					glog.Errorln(err)
				}

			} else {
				glog.Errorf(constantes.ErrUnableToDeleteNode, node.Name, nodeGroup.NodeGroupIdentifier)
			}
		}
	}()

	return &externalgrpc.NodeGroupDeleteNodesResponse{}, nil
}

// NodeGroupDecreaseTargetSize decreases the target size of the node group. This function
// doesn't permit to delete any existing node and can be used only to reduce the request
// for new nodes that have not been yet fulfilled. Delta should be negative. It is assumed
// that cloud provider will not delete the existing nodes if the size when there is an option
// to just decrease the target.
func (v *externalgrpcServerApp) NodeGroupDecreaseTargetSize(ctx context.Context, request *externalgrpc.NodeGroupDecreaseTargetSizeRequest) (*externalgrpc.NodeGroupDecreaseTargetSizeResponse, error) {
	glog.Debugf("Call server DecreaseTargetSize: %v", request)

	nodeGroup := v.appServer.Groups[request.GetId()]

	if nodeGroup == nil {
		glog.Errorf(constantes.ErrNodeGroupNotFound, request.GetId())

		return nil, fmt.Errorf(constantes.ErrNodeGroupNotFound, request.GetId())
	}

	if request.GetDelta() >= 0 {
		glog.Errorf(constantes.ErrDecreaseSizeMustBeNegative)

		return nil, fmt.Errorf(constantes.ErrDecreaseSizeMustBeNegative)
	}

	newSize := nodeGroup.targetSize() + int(request.GetDelta())

	if newSize < len(nodeGroup.Nodes) {
		glog.Errorf(constantes.ErrDecreaseSizeAttemptDeleteNodes, nodeGroup.targetSize(), request.GetDelta(), newSize)

		return nil, fmt.Errorf(constantes.ErrDecreaseSizeAttemptDeleteNodes, nodeGroup.targetSize(), request.GetDelta(), newSize)
	}

	go nodeGroup.setNodeGroupSize(v.appServer.kubeClient, newSize)

	return &externalgrpc.NodeGroupDecreaseTargetSizeResponse{}, nil
}

// NodeGroupNodes returns a list of all nodes that belong to this node group.
func (v *externalgrpcServerApp) NodeGroupNodes(ctx context.Context, request *externalgrpc.NodeGroupNodesRequest) (*externalgrpc.NodeGroupNodesResponse, error) {
	glog.Debugf("Call server Nodes: %v", request)

	nodeGroup := v.appServer.Groups[request.GetId()]

	if nodeGroup == nil {
		glog.Errorf(constantes.ErrNodeGroupNotFound, request.GetId())

		return nil, fmt.Errorf(constantes.ErrNodeGroupNotFound, request.GetId())
	}

	nodes := nodeGroup.AllNodes()
	instances := make([]*externalgrpc.Instance, 0, len(nodes))

	for _, node := range nodes {
		status := externalgrpc.InstanceStatus_unspecified

		switch node.State {
		case AutoScalerServerNodeStateRunning, AutoScalerServerNodeStateStopped:
			status = externalgrpc.InstanceStatus_instanceRunning
		case AutoScalerServerNodeStateCreating:
			status = externalgrpc.InstanceStatus_instanceCreating
		case AutoScalerServerNodeStateDeleted:
			status = externalgrpc.InstanceStatus_instanceDeleting
		}

		instances = append(instances, &externalgrpc.Instance{
			Id: node.generateProviderID(),
			Status: &externalgrpc.InstanceStatus{
				InstanceState: status,
			},
		})
	}

	return &externalgrpc.NodeGroupNodesResponse{
		Instances: instances,
	}, nil
}

// NodeGroupTemplateNodeInfo returns a structure of an empty (as if just started) node,
// with all of the labels, capacity and allocatable information. This will be used in
// scale-up simulations to predict what would a new node look like if a node group was expanded.
// Implementation optional.
func (v *externalgrpcServerApp) NodeGroupTemplateNodeInfo(ctx context.Context, request *externalgrpc.NodeGroupTemplateNodeInfoRequest) (*externalgrpc.NodeGroupTemplateNodeInfoResponse, error) {
	glog.Debugf("Call server TemplateNodeInfo: %v", request)

	nodeGroup := v.appServer.Groups[request.GetId()]

	if nodeGroup == nil {
		glog.Errorf(constantes.ErrNodeGroupNotFound, request.GetId())

		return nil, fmt.Errorf(constantes.ErrNodeGroupNotFound, request.GetId())
	}

	labels := utils.MergeKubernetesLabel(nodeGroup.NodeLabels, nodeGroup.SystemLabels)
	annotations := types.KubernetesLabel{
		constantes.AnnotationNodeGroupName:        request.GetId(),
		constantes.AnnotationScaleDownDisabled:    "false",
		constantes.AnnotationNodeAutoProvisionned: "true",
		constantes.AnnotationNodeManaged:          "false",
	}

	return &externalgrpc.NodeGroupTemplateNodeInfoResponse{
		NodeInfo: &v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Labels:      labels,
				Annotations: annotations,
			},
			Spec: v1.NodeSpec{
				Unschedulable: false,
			},
		},
	}, nil
}

// GetOptions returns NodeGroupAutoscalingOptions that should be used for this particular
// NodeGroup. Returning a grpc error will result in using default options.
// Implementation optional.
func (v *externalgrpcServerApp) NodeGroupGetOptions(ctx context.Context, request *externalgrpc.NodeGroupAutoscalingOptionsRequest) (*externalgrpc.NodeGroupAutoscalingOptionsResponse, error) {
	nodeGroup := v.appServer.Groups[request.GetId()]

	if nodeGroup == nil {
		glog.Errorf(constantes.ErrNodeGroupNotFound, request.GetId())

		return nil, fmt.Errorf(constantes.ErrNodeGroupNotFound, request.GetId())
	}

	pbDefaults := request.GetDefaults()

	if pbDefaults == nil {
		return nil, fmt.Errorf("request fields were nil")
	}

	defaults := &types.NodeGroupAutoscalingOptions{
		ScaleDownUtilizationThreshold:    pbDefaults.GetScaleDownGpuUtilizationThreshold(),
		ScaleDownGpuUtilizationThreshold: pbDefaults.GetScaleDownGpuUtilizationThreshold(),
		ScaleDownUnneededTime:            pbDefaults.GetScaleDownUnneededTime().Duration,
		ScaleDownUnreadyTime:             pbDefaults.GetScaleDownUnneededTime().Duration,
	}

	opts, err := nodeGroup.GetOptions(defaults)

	if err != nil {
		return nil, err
	}

	if opts == nil {
		return nil, fmt.Errorf("GetOptions not implemented") //make this explicitly so that grpc response is discarded
	}

	return &externalgrpc.NodeGroupAutoscalingOptionsResponse{
		NodeGroupAutoscalingOptions: &externalgrpc.NodeGroupAutoscalingOptions{
			ScaleDownUtilizationThreshold:    opts.ScaleDownUtilizationThreshold,
			ScaleDownGpuUtilizationThreshold: opts.ScaleDownGpuUtilizationThreshold,
			ScaleDownUnneededTime: &metav1.Duration{
				Duration: opts.ScaleDownUnneededTime,
			},
			ScaleDownUnreadyTime: &metav1.Duration{
				Duration: opts.ScaleDownUnreadyTime,
			},
		},
	}, nil
}
