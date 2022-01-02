package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/Fred78290/kubernetes-vmware-autoscaler/constantes"
	apigrpc "github.com/Fred78290/kubernetes-vmware-autoscaler/grpc"
	"github.com/Fred78290/kubernetes-vmware-autoscaler/pkg/signals"
	"github.com/Fred78290/kubernetes-vmware-autoscaler/types"
	"github.com/Fred78290/kubernetes-vmware-autoscaler/utils"
	glog "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	apiv1 "k8s.io/api/core/v1"
)

type applicationInterface interface {
	getNodeGroup(nodegroup string) (*AutoScalerServerNodeGroup, error)
	isNodegroupDiscovered() bool
	getResourceLimiter() *types.ResourceLimiter
	syncState()
	client() types.ClientGenerator
}

// AutoScalerServerApp declare AutoScaler grpc server
type AutoScalerServerApp struct {
	ResourceLimiter *types.ResourceLimiter                `json:"limits"`
	Groups          map[string]*AutoScalerServerNodeGroup `json:"groups"`
	NodesDefinition []*apigrpc.NodeGroupDef               `json:"nodedefs"`
	AutoProvision   bool                                  `json:"auto"`
	configuration   *types.AutoScalerServerConfig
	running         bool
	kubeClient      types.ClientGenerator
	requestTimeout  time.Duration
}

var phSavedState = ""
var phSaveState bool

func (s *AutoScalerServerApp) generateNodeGroupName() string {
	return fmt.Sprintf("ng-%d", time.Now().Unix())
}

func (s *AutoScalerServerApp) isNodegroupDiscovered() bool {
	return len(s.Groups) > 0
}

func (s *AutoScalerServerApp) getResourceLimiter() *types.ResourceLimiter {
	return s.ResourceLimiter
}

func (s *AutoScalerServerApp) getNodeGroup(nodegroupName string) (*AutoScalerServerNodeGroup, error) {
	if ng, found := s.Groups[nodegroupName]; found {
		return ng, nil
	}

	return nil, fmt.Errorf(constantes.ErrNodeGroupNotFound, nodegroupName)
}

func (s *AutoScalerServerApp) newNodeGroup(nodeGroupID string, minNodeSize, maxNodeSize int32, machineType string, labels, systemLabels KubernetesLabel, autoProvision bool) (*AutoScalerServerNodeGroup, error) {

	machine := s.configuration.Machines[machineType]

	if machine == nil {
		return nil, fmt.Errorf(constantes.ErrMachineTypeNotFound, machineType)
	}

	if nodeGroup := s.Groups[nodeGroupID]; nodeGroup != nil {
		glog.Errorf(constantes.ErrNodeGroupAlreadyExists, nodeGroupID)

		return nil, fmt.Errorf(constantes.ErrNodeGroupAlreadyExists, nodeGroupID)
	}

	glog.Infof("New node group, ID:%s minSize:%d, maxSize:%d, machineType:%s, node lables:%v, %v", nodeGroupID, minNodeSize, maxNodeSize, machineType, labels, systemLabels)

	nodeGroup := &AutoScalerServerNodeGroup{
		ServiceIdentifier:          s.configuration.ProviderID,
		ProvisionnedNodeNamePrefix: s.configuration.ProvisionnedNodeNamePrefix,
		ManagedNodeNamePrefix:      s.configuration.ManagedNodeNamePrefix,
		ControlPlaneNamePrefix:     s.configuration.ControlPlaneNamePrefix,
		NodeGroupIdentifier:        nodeGroupID,
		Machine:                    machine,
		Status:                     NodegroupNotCreated,
		pendingNodes:               make(map[string]*AutoScalerServerNode),
		Nodes:                      make(map[string]*AutoScalerServerNode),
		MinNodeSize:                int(minNodeSize),
		MaxNodeSize:                int(maxNodeSize),
		NodeLabels:                 labels,
		SystemLabels:               systemLabels,
		AutoProvision:              autoProvision,
		configuration:              s.configuration,
	}

	s.Groups[nodeGroupID] = nodeGroup

	return nodeGroup, nil
}

func (s *AutoScalerServerApp) deleteNodeGroup(nodeGroupID string) error {
	nodeGroup := s.Groups[nodeGroupID]

	if nodeGroup == nil {
		glog.Errorf(constantes.ErrNodeGroupNotFound, nodeGroupID)
		return fmt.Errorf(constantes.ErrNodeGroupNotFound, nodeGroupID)
	}

	glog.Infof("Delete node group, ID:%s", nodeGroupID)

	if err := nodeGroup.deleteNodeGroup(s.kubeClient); err != nil {
		glog.Errorf(constantes.ErrUnableToDeleteNodeGroup, nodeGroupID, err)
		return err
	}

	delete(s.Groups, nodeGroupID)

	return nil
}

func (s *AutoScalerServerApp) createNodeGroup(nodeGroupID string) (*AutoScalerServerNodeGroup, error) {
	nodeGroup := s.Groups[nodeGroupID]

	if nodeGroup == nil {
		glog.Errorf(constantes.ErrNodeGroupNotFound, nodeGroupID)
		return nil, fmt.Errorf(constantes.ErrNodeGroupNotFound, nodeGroupID)
	}

	if nodeGroup.Status == NodegroupNotCreated {
		// Must launch minNode VM
		if nodeGroup.MinNodeSize > 0 {

			glog.Infof("Create node group, ID:%s", nodeGroupID)

			if _, err := nodeGroup.addNodes(s.kubeClient, nodeGroup.MinNodeSize); err != nil {
				glog.Errorf(err.Error())

				return nodeGroup, err
			}
		}

		nodeGroup.Status = NodegroupCreated
	}

	return nodeGroup, nil
}

func (s *AutoScalerServerApp) doAutoProvision() error {
	glog.Debug("Call server doAutoProvision")

	var ng *AutoScalerServerNodeGroup
	var err error

	for _, nodeGroupDef := range s.NodesDefinition {
		nodeGroupIdentifier := nodeGroupDef.GetNodeGroupID()

		if len(nodeGroupIdentifier) > 0 {
			ng = s.Groups[nodeGroupIdentifier]

			if ng == nil {
				systemLabels := map[string]string{
					constantes.NodeLabelWorkerRole: "",
				}

				labels := map[string]string{
					constantes.NodeLabelGroupName: nodeGroupIdentifier,
				}

				// Default labels
				if nodeGroupDef.GetLabels() != nil {
					for k, v := range nodeGroupDef.GetLabels() {
						labels[k] = v
					}
				}

				glog.Infof("Auto provision for nodegroup:%s, minSize:%d, maxSize:%d", nodeGroupIdentifier, nodeGroupDef.MinSize, nodeGroupDef.MaxSize)

				if _, err = s.newNodeGroup(nodeGroupIdentifier, nodeGroupDef.MinSize, nodeGroupDef.MaxSize, s.configuration.DefaultMachineType, labels, systemLabels, true); err == nil {
					if ng, err = s.createNodeGroup(nodeGroupIdentifier); err == nil {
						if err = ng.autoDiscoveryNodes(s.kubeClient, nodeGroupDef.GetIncludeExistingNode()); err == nil {
							return err
						}
					}
				}

				if err != nil {
					break
				}
			} else {
				// If the nodegroup already exists, reparse nodes
				if err = ng.autoDiscoveryNodes(s.kubeClient, nodeGroupDef.GetIncludeExistingNode()); err == nil {
					return err
				}
			}
		}
	}

	return err
}

// Connect allows client to connect
func (s *AutoScalerServerApp) Connect(ctx context.Context, request *apigrpc.ConnectRequest) (*apigrpc.ConnectReply, error) {
	glog.Debugf("Call server Connect: %v", request)

	if request.GetProviderID() != s.configuration.ProviderID {
		glog.Errorf(constantes.ErrMismatchingProvider)

		return nil, fmt.Errorf(constantes.ErrMismatchingProvider)
	}

	if request.GetResourceLimiter() != nil {
		if s.ResourceLimiter != nil {
			s.ResourceLimiter.MergeRequestResourceLimiter(request.GetResourceLimiter())
		} else {
			s.ResourceLimiter = &types.ResourceLimiter{
				MinLimits: request.ResourceLimiter.MinLimits,
				MaxLimits: request.ResourceLimiter.MaxLimits,
			}

			s.ResourceLimiter.SetMaxValue(constantes.ResourceNameNodes, s.configuration.MaxNode)
			s.ResourceLimiter.SetMinValue(constantes.ResourceNameNodes, s.configuration.MinNode)
		}
	}

	s.NodesDefinition = request.GetNodes()
	s.AutoProvision = request.GetAutoProvisionned()

	if s.AutoProvision {
		if err := s.doAutoProvision(); err != nil {
			glog.Errorf(constantes.ErrUnableToAutoProvisionNodeGroup, err)

			return nil, err
		}
	}

	return &apigrpc.ConnectReply{
		Response: &apigrpc.ConnectReply_Connected{
			Connected: true,
		},
	}, nil
}

// Name returns name of the cloud provider.
func (s *AutoScalerServerApp) Name(ctx context.Context, request *apigrpc.CloudProviderServiceRequest) (*apigrpc.NameReply, error) {
	glog.Debugf("Call server Name: %v", request)

	if request.GetProviderID() != s.configuration.ProviderID {
		glog.Errorf(constantes.ErrMismatchingProvider)
		return nil, fmt.Errorf(constantes.ErrMismatchingProvider)
	}

	return &apigrpc.NameReply{
		Name: constantes.ProviderName,
	}, nil
}

// NodeGroups returns all node groups configured for this cloud provider.
func (s *AutoScalerServerApp) NodeGroups(ctx context.Context, request *apigrpc.CloudProviderServiceRequest) (*apigrpc.NodeGroupsReply, error) {
	glog.Debugf("Call server NodeGroups: %v", request)

	if request.GetProviderID() != s.configuration.ProviderID {
		glog.Errorf(constantes.ErrMismatchingProvider)
		return nil, fmt.Errorf(constantes.ErrMismatchingProvider)
	}

	nodeGroups := make([]*apigrpc.NodeGroup, 0, len(s.Groups))

	for name, nodeGroup := range s.Groups {
		// Return node group if created
		if nodeGroup.Status == NodegroupCreated {
			nodeGroups = append(nodeGroups, &apigrpc.NodeGroup{
				Id: name,
			})
		}
	}

	return &apigrpc.NodeGroupsReply{
		NodeGroups: nodeGroups,
	}, nil
}

func (s *AutoScalerServerApp) nodeGroupForNode(providerID string) (*AutoScalerServerNodeGroup, error) {
	nodeGroupID, err := utils.NodeGroupIDFromProviderID(s.configuration.ProviderID, providerID)

	if err != nil {
		glog.Errorf(constantes.ErrCantDecodeNodeIDWithReason, providerID, err)
		return nil, fmt.Errorf(constantes.ErrCantDecodeNodeIDWithReason, providerID, err)
	}

	if len(nodeGroupID) == 0 {
		glog.Errorf(constantes.ErrCantDecodeNodeID, providerID)
		return nil, fmt.Errorf(constantes.ErrCantDecodeNodeID, providerID)
	}

	nodeGroup := s.Groups[nodeGroupID]

	if nodeGroup == nil {
		glog.Errorf(constantes.ErrNodeGroupForNodeNotFound, nodeGroupID, providerID)

		//return nil, fmt.Errorf(errNodeGroupForNodeNotFound, nodeGroupID, providerID)
	}

	return nodeGroup, err
}

// NodeGroupForNode returns the node group for the given node, nil if the node
// should not be processed by cluster autoscaler, or non-nil error if such
// occurred. Must be implemented.
func (s *AutoScalerServerApp) NodeGroupForNode(ctx context.Context, request *apigrpc.NodeGroupForNodeRequest) (*apigrpc.NodeGroupForNodeReply, error) {
	glog.Debugf("Call server NodeGroupForNode: %v", request)

	if request.GetProviderID() != s.configuration.ProviderID {
		glog.Errorf(constantes.ErrMismatchingProvider)
		return nil, fmt.Errorf(constantes.ErrMismatchingProvider)
	}

	node, err := utils.NodeFromJSON(request.GetNode())

	if err != nil {
		glog.Errorf(constantes.ErrCantUnmarshallNodeWithReason, request.GetNode(), err)

		return &apigrpc.NodeGroupForNodeReply{
			Response: &apigrpc.NodeGroupForNodeReply_Error{
				Error: &apigrpc.Error{
					Code:   constantes.CloudProviderError,
					Reason: err.Error(),
				},
			},
		}, nil
	}

	providerID := utils.GetNodeProviderID(s.configuration.ProviderID, node)

	if len(providerID) == 0 {
		glog.Debug("node.Spec.ProviderID is empty")
		return &apigrpc.NodeGroupForNodeReply{
			Response: &apigrpc.NodeGroupForNodeReply_NodeGroup{
				NodeGroup: &apigrpc.NodeGroup{},
			},
		}, nil
	}

	nodeGroup, err := s.nodeGroupForNode(providerID)

	if err != nil {
		return &apigrpc.NodeGroupForNodeReply{
			Response: &apigrpc.NodeGroupForNodeReply_Error{
				Error: &apigrpc.Error{
					Code:   constantes.CloudProviderError,
					Reason: err.Error(),
				},
			},
		}, nil
	}

	if nodeGroup == nil {
		glog.Infof("Nodegroup not found for node.Spec.ProviderID:%s", providerID)

		return &apigrpc.NodeGroupForNodeReply{
			Response: &apigrpc.NodeGroupForNodeReply_NodeGroup{
				NodeGroup: &apigrpc.NodeGroup{},
			},
		}, nil
	}

	return &apigrpc.NodeGroupForNodeReply{
		Response: &apigrpc.NodeGroupForNodeReply_NodeGroup{
			NodeGroup: &apigrpc.NodeGroup{
				Id: nodeGroup.NodeGroupIdentifier,
			},
		},
	}, nil
}

// Pricing returns pricing model for this cloud provider or error if not available.
// Implementation optional.
func (s *AutoScalerServerApp) Pricing(ctx context.Context, request *apigrpc.CloudProviderServiceRequest) (*apigrpc.PricingModelReply, error) {
	glog.Debugf("Call server Pricing: %v", request)

	if s.configuration.Optionals.Pricing {
		return nil, fmt.Errorf(constantes.ErrNotImplemented)
	}

	if request.GetProviderID() != s.configuration.ProviderID {
		glog.Errorf(constantes.ErrMismatchingProvider)
		return nil, fmt.Errorf(constantes.ErrMismatchingProvider)
	}

	return &apigrpc.PricingModelReply{
		Response: &apigrpc.PricingModelReply_PriceModel{
			PriceModel: &apigrpc.PricingModel{
				Id: s.configuration.ProviderID,
			},
		},
	}, nil
}

// GetAvailableMachineTypes get all machine types that can be requested from the cloud provider.
// Implementation optional.
func (s *AutoScalerServerApp) GetAvailableMachineTypes(ctx context.Context, request *apigrpc.CloudProviderServiceRequest) (*apigrpc.AvailableMachineTypesReply, error) {
	glog.Debugf("Call server GetAvailableMachineTypes: %v", request)

	if s.configuration.Optionals.GetAvailableMachineTypes {
		return nil, fmt.Errorf(constantes.ErrNotImplemented)
	}

	if request.GetProviderID() != s.configuration.ProviderID {
		glog.Errorf(constantes.ErrMismatchingProvider)
		return nil, fmt.Errorf(constantes.ErrMismatchingProvider)
	}

	machineTypes := make([]string, 0, len(s.configuration.Machines))

	for n := range s.configuration.Machines {
		machineTypes = append(machineTypes, n)
	}

	return &apigrpc.AvailableMachineTypesReply{
		Response: &apigrpc.AvailableMachineTypesReply_AvailableMachineTypes{
			AvailableMachineTypes: &apigrpc.AvailableMachineTypes{
				MachineType: machineTypes,
			},
		},
	}, nil
}

// NewNodeGroup builds a theoretical node group based on the node definition provided. The node group is not automatically
// created on the cloud provider side. The node group is not returned by NodeGroups() until it is created.
// Implementation optional.
func (s *AutoScalerServerApp) NewNodeGroup(ctx context.Context, request *apigrpc.NewNodeGroupRequest) (*apigrpc.NewNodeGroupReply, error) {
	glog.Debugf("Call server NewNodeGroup: %v", request)

	if s.configuration.Optionals.NewNodeGroup {
		return nil, fmt.Errorf(constantes.ErrNotImplemented)
	}

	if request.GetProviderID() != s.configuration.ProviderID {
		glog.Errorf(constantes.ErrMismatchingProvider)
		return nil, fmt.Errorf(constantes.ErrMismatchingProvider)
	}

	machineType := s.configuration.Machines[request.GetMachineType()]

	if machineType == nil {
		glog.Errorf(constantes.ErrMachineTypeNotFound, request.GetMachineType())

		return &apigrpc.NewNodeGroupReply{
			Response: &apigrpc.NewNodeGroupReply_Error{
				Error: &apigrpc.Error{
					Code:   constantes.CloudProviderError,
					Reason: fmt.Sprintf(constantes.ErrMachineTypeNotFound, request.GetMachineType()),
				},
			},
		}, nil
	}

	var nodeGroupIdentifier string

	labels := make(map[string]string)
	systemLabels := make(map[string]string)

	systemLabels[constantes.NodeLabelWorkerRole] = ""

	if reqLabels := request.GetLabels(); reqLabels != nil {
		for k2, v2 := range reqLabels {
			labels[k2] = v2
		}
	}

	if reqSystemLabels := request.GetSystemLabels(); reqSystemLabels != nil {
		for k2, v2 := range reqSystemLabels {
			systemLabels[k2] = v2
		}
	}

	if len(request.GetNodeGroupID()) == 0 {
		nodeGroupIdentifier = s.generateNodeGroupName()
	} else {
		nodeGroupIdentifier = request.GetNodeGroupID()
	}

	labels[constantes.NodeLabelGroupName] = nodeGroupIdentifier

	nodeGroup, err := s.newNodeGroup(nodeGroupIdentifier, request.GetMinNodeSize(), request.GetMaxNodeSize(), request.GetMachineType(), labels, systemLabels, false)

	if err != nil {
		glog.Errorf(constantes.ErrUnableToCreateNodeGroup, nodeGroupIdentifier, err)

		return &apigrpc.NewNodeGroupReply{
			Response: &apigrpc.NewNodeGroupReply_Error{
				Error: &apigrpc.Error{
					Code:   constantes.CloudProviderError,
					Reason: fmt.Sprintf(constantes.ErrUnableToCreateNodeGroup, nodeGroupIdentifier, err),
				},
			},
		}, nil
	}

	return &apigrpc.NewNodeGroupReply{
		Response: &apigrpc.NewNodeGroupReply_NodeGroup{
			NodeGroup: &apigrpc.NodeGroup{
				Id: nodeGroup.NodeGroupIdentifier,
			},
		},
	}, nil
}

// GetResourceLimiter returns struct containing limits (max, min) for resources (cores, memory etc.).
func (s *AutoScalerServerApp) GetResourceLimiter(ctx context.Context, request *apigrpc.CloudProviderServiceRequest) (*apigrpc.ResourceLimiterReply, error) {
	glog.Debugf("Call server GetResourceLimiter: %v", request)

	if request.GetProviderID() != s.configuration.ProviderID {
		glog.Errorf(constantes.ErrMismatchingProvider)
		return nil, fmt.Errorf(constantes.ErrMismatchingProvider)
	}

	return &apigrpc.ResourceLimiterReply{
		Response: &apigrpc.ResourceLimiterReply_ResourceLimiter{
			ResourceLimiter: &apigrpc.ResourceLimiter{
				MinLimits: s.ResourceLimiter.MinLimits,
				MaxLimits: s.ResourceLimiter.MaxLimits,
			},
		},
	}, nil
}

// GPULabel returns the label added to nodes with GPU resource.
func (s *AutoScalerServerApp) GPULabel(ctx context.Context, request *apigrpc.CloudProviderServiceRequest) (*apigrpc.GPULabelReply, error) {
	glog.Debugf("Call server GPULabel: %v", request)

	if request.GetProviderID() != s.configuration.ProviderID {
		glog.Errorf(constantes.ErrMismatchingProvider)
		return nil, fmt.Errorf(constantes.ErrMismatchingProvider)
	}

	return &apigrpc.GPULabelReply{
		Response: &apigrpc.GPULabelReply_Gpulabel{
			Gpulabel: "",
		},
	}, nil
}

// GetAvailableGPUTypes return all available GPU types cloud provider supports.
func (s *AutoScalerServerApp) GetAvailableGPUTypes(ctx context.Context, request *apigrpc.CloudProviderServiceRequest) (*apigrpc.GetAvailableGPUTypesReply, error) {
	glog.Debugf("Call server GetAvailableGPUTypes: %v", request)

	if request.GetProviderID() != s.configuration.ProviderID {
		glog.Errorf(constantes.ErrMismatchingProvider)
		return nil, fmt.Errorf(constantes.ErrMismatchingProvider)
	}

	gpus := make(map[string]string)

	return &apigrpc.GetAvailableGPUTypesReply{
		AvailableGpuTypes: gpus,
	}, nil
}

// Cleanup cleans up open resources before the cloud provider is destroyed, i.e. go routines etc.
func (s *AutoScalerServerApp) Cleanup(ctx context.Context, request *apigrpc.CloudProviderServiceRequest) (*apigrpc.CleanupReply, error) {
	glog.Debugf("Call server Cleanup: %v", request)

	var lastError *apigrpc.Error

	if request.GetProviderID() != s.configuration.ProviderID {
		glog.Errorf(constantes.ErrMismatchingProvider)
		return nil, fmt.Errorf(constantes.ErrMismatchingProvider)
	}

	for _, nodeGroup := range s.Groups {
		if err := nodeGroup.cleanup(s.kubeClient); err != nil {
			lastError = &apigrpc.Error{
				Code:   constantes.CloudProviderError,
				Reason: err.Error(),
			}
		}
	}

	glog.Debug("Leave server Cleanup, done")

	s.Groups = make(map[string]*AutoScalerServerNodeGroup)

	return &apigrpc.CleanupReply{
		Error: lastError,
	}, nil
}

func (s *AutoScalerServerApp) syncState() {
	for _, ng := range s.Groups {
		ng.refresh()
	}

	if phSaveState {
		if err := s.Save(phSavedState); err != nil {
			glog.Errorf(constantes.ErrFailedToSaveServerState, err)
		}
	}
}

// Refresh is called before every main loop and can be used to dynamically update cloud provider state.
// In particular the list of node groups returned by NodeGroups can change as a result of CloudProvider.Refresh().
func (s *AutoScalerServerApp) Refresh(ctx context.Context, request *apigrpc.CloudProviderServiceRequest) (*apigrpc.RefreshReply, error) {
	glog.Debugf("Call server Refresh: %v", request)

	if request.GetProviderID() != s.configuration.ProviderID {
		glog.Errorf(constantes.ErrMismatchingProvider)
		return nil, fmt.Errorf(constantes.ErrMismatchingProvider)
	}

	for _, ng := range s.Groups {
		ng.refresh()
	}

	if phSaveState {
		if err := s.Save(phSavedState); err != nil {
			glog.Errorf(constantes.ErrFailedToSaveServerState, err)
		}
	}

	return &apigrpc.RefreshReply{
		Error: nil,
	}, nil
}

// MaxSize returns maximum size of the node group.
func (s *AutoScalerServerApp) MaxSize(ctx context.Context, request *apigrpc.NodeGroupServiceRequest) (*apigrpc.MaxSizeReply, error) {
	glog.Debugf("Call server MaxSize: %v", request)

	var maxSize int

	if request.GetProviderID() != s.configuration.ProviderID {
		glog.Errorf(constantes.ErrMismatchingProvider)
		return nil, fmt.Errorf(constantes.ErrMismatchingProvider)
	}

	nodeGroup := s.Groups[request.GetNodeGroupID()]

	if nodeGroup == nil {
		glog.Errorf(constantes.ErrNodeGroupNotFound, request.GetNodeGroupID())
	} else {
		maxSize = nodeGroup.MaxNodeSize
	}

	return &apigrpc.MaxSizeReply{
		MaxSize: int32(maxSize),
	}, nil
}

// MinSize returns minimum size of the node group.
func (s *AutoScalerServerApp) MinSize(ctx context.Context, request *apigrpc.NodeGroupServiceRequest) (*apigrpc.MinSizeReply, error) {
	glog.Debugf("Call server MinSize: %v", request)

	var minSize int

	if request.GetProviderID() != s.configuration.ProviderID {
		return nil, fmt.Errorf(constantes.ErrMismatchingProvider)
	}

	nodeGroup := s.Groups[request.GetNodeGroupID()]

	if nodeGroup == nil {
		glog.Errorf(constantes.ErrNodeGroupNotFound, request.GetNodeGroupID())
	} else {
		minSize = nodeGroup.MinNodeSize
	}

	return &apigrpc.MinSizeReply{
		MinSize: int32(minSize),
	}, nil
}

// TargetSize returns the current target size of the node group. It is possible that the
// number of nodes in Kubernetes is different at the moment but should be equal
// to Size() once everything stabilizes (new nodes finish startup and registration or
// removed nodes are deleted completely). Implementation required.
func (s *AutoScalerServerApp) TargetSize(ctx context.Context, request *apigrpc.NodeGroupServiceRequest) (*apigrpc.TargetSizeReply, error) {
	glog.Debugf("Call server TargetSize: %v", request)

	if request.GetProviderID() != s.configuration.ProviderID {
		glog.Errorf(constantes.ErrMismatchingProvider)
		return nil, fmt.Errorf(constantes.ErrMismatchingProvider)
	}

	nodeGroup := s.Groups[request.GetNodeGroupID()]

	if nodeGroup == nil {
		glog.Errorf(constantes.ErrNodeGroupNotFound, request.GetNodeGroupID())

		return &apigrpc.TargetSizeReply{
			Response: &apigrpc.TargetSizeReply_Error{
				Error: &apigrpc.Error{
					Code:   constantes.CloudProviderError,
					Reason: fmt.Sprintf(constantes.ErrNodeGroupNotFound, request.GetNodeGroupID()),
				},
			},
		}, nil
	}

	return &apigrpc.TargetSizeReply{
		Response: &apigrpc.TargetSizeReply_TargetSize{
			TargetSize: int32(nodeGroup.targetSize()),
		},
	}, nil
}

// IncreaseSize increases the size of the node group. To delete a node you need
// to explicitly name it and use DeleteNode. This function should wait until
// node group size is updated. Implementation required.
func (s *AutoScalerServerApp) IncreaseSize(ctx context.Context, request *apigrpc.IncreaseSizeRequest) (*apigrpc.IncreaseSizeReply, error) {
	glog.Debugf("Call server IncreaseSize: %v", request)

	if request.GetProviderID() != s.configuration.ProviderID {
		glog.Errorf(constantes.ErrMismatchingProvider)
		return nil, fmt.Errorf(constantes.ErrMismatchingProvider)
	}

	nodeGroup := s.Groups[request.GetNodeGroupID()]

	if nodeGroup == nil {
		glog.Errorf(constantes.ErrNodeGroupNotFound, request.GetNodeGroupID())

		return &apigrpc.IncreaseSizeReply{
			Error: &apigrpc.Error{
				Code:   constantes.CloudProviderError,
				Reason: fmt.Sprintf(constantes.ErrNodeGroupNotFound, request.GetNodeGroupID()),
			},
		}, nil
	}

	if request.GetDelta() <= 0 {
		glog.Errorf(constantes.ErrIncreaseSizeMustBePositive)

		return &apigrpc.IncreaseSizeReply{
			Error: &apigrpc.Error{
				Code:   constantes.CloudProviderError,
				Reason: constantes.ErrIncreaseSizeMustBePositive,
			},
		}, nil
	}

	newSize := len(nodeGroup.Nodes) + int(request.GetDelta())

	if newSize > nodeGroup.MaxNodeSize {
		glog.Errorf(constantes.ErrIncreaseSizeTooLarge, newSize, nodeGroup.MaxNodeSize)

		return &apigrpc.IncreaseSizeReply{
			Error: &apigrpc.Error{
				Code:   constantes.CloudProviderError,
				Reason: fmt.Sprintf(constantes.ErrIncreaseSizeTooLarge, newSize, nodeGroup.MaxNodeSize),
			},
		}, nil
	}

	err := nodeGroup.setNodeGroupSize(s.kubeClient, newSize)

	if err != nil {
		return &apigrpc.IncreaseSizeReply{
			Error: &apigrpc.Error{
				Code:   constantes.CloudProviderError,
				Reason: err.Error(),
			},
		}, nil
	}

	return &apigrpc.IncreaseSizeReply{
		Error: nil,
	}, nil
}

// DeleteNodes deletes nodes from this node group. Error is returned either on
// failure or if the given node doesn't belong to this node group. This function
// should wait until node group size is updated. Implementation required.
func (s *AutoScalerServerApp) DeleteNodes(ctx context.Context, request *apigrpc.DeleteNodesRequest) (*apigrpc.DeleteNodesReply, error) {
	glog.Debugf("Call server DeleteNodes: %v", request)

	if request.GetProviderID() != s.configuration.ProviderID {
		glog.Errorf(constantes.ErrMismatchingProvider)
		return nil, fmt.Errorf(constantes.ErrMismatchingProvider)
	}

	nodeGroup := s.Groups[request.GetNodeGroupID()]

	if nodeGroup == nil {
		glog.Errorf(constantes.ErrNodeGroupNotFound, request.GetNodeGroupID())

		return &apigrpc.DeleteNodesReply{
			Error: &apigrpc.Error{
				Code:   constantes.CloudProviderError,
				Reason: fmt.Sprintf(constantes.ErrNodeGroupNotFound, request.GetNodeGroupID()),
			},
		}, nil
	}

	if nodeGroup.targetSize()-len(request.GetNode()) < nodeGroup.MinNodeSize {
		return &apigrpc.DeleteNodesReply{
			Error: &apigrpc.Error{
				Code:   constantes.CloudProviderError,
				Reason: fmt.Sprintf(constantes.ErrMinSizeReached, request.GetNodeGroupID()),
			},
		}, nil
	}

	// Iterate over each requested node to delete
	for idx, sNode := range request.GetNode() {
		node, err := utils.NodeFromJSON(sNode)

		// Can't deserialize
		if node == nil || err != nil {
			glog.Errorf(constantes.ErrCantUnmarshallNodeWithReason, sNode, err)

			return &apigrpc.DeleteNodesReply{
				Error: &apigrpc.Error{
					Code:   constantes.CloudProviderError,
					Reason: fmt.Sprintf(constantes.ErrCantUnmarshallNode, idx, request.GetNodeGroupID()),
				},
			}, nil
		}

		// Check node group owner
		nodeName := utils.GetNodeProviderID(s.configuration.ProviderID, node)
		nodeGroupForNode, err := s.nodeGroupForNode(nodeName)

		// Node group not found
		if err != nil {
			glog.Errorf(constantes.ErrNodeGroupNotFound, nodeName)

			return &apigrpc.DeleteNodesReply{
				Error: &apigrpc.Error{
					Code:   constantes.CloudProviderError,
					Reason: err.Error(),
				},
			}, nil
		}

		// Not in the same group
		if nodeGroupForNode.NodeGroupIdentifier != nodeGroup.NodeGroupIdentifier {
			return &apigrpc.DeleteNodesReply{
				Error: &apigrpc.Error{
					Code:   constantes.CloudProviderError,
					Reason: fmt.Sprintf(constantes.ErrUnableToDeleteNode, nodeName, nodeGroup.NodeGroupIdentifier),
				},
			}, nil
		}

		// Delete the node in the group
		nodeName, _ = utils.NodeNameFromProviderID(s.configuration.ProviderID, nodeName)

		err = nodeGroup.deleteNodeByName(s.kubeClient, nodeName)

		if err != nil {
			return &apigrpc.DeleteNodesReply{
				Error: &apigrpc.Error{
					Code:   constantes.CloudProviderError,
					Reason: err.Error(),
				},
			}, nil
		}
	}

	return &apigrpc.DeleteNodesReply{
		Error: nil,
	}, nil
}

// DecreaseTargetSize decreases the target size of the node group. This function
// doesn't permit to delete any existing node and can be used only to reduce the
// request for new nodes that have not been yet fulfilled. Delta should be negative.
// It is assumed that cloud provider will not delete the existing nodes when there
// is an option to just decrease the target. Implementation required.
func (s *AutoScalerServerApp) DecreaseTargetSize(ctx context.Context, request *apigrpc.DecreaseTargetSizeRequest) (*apigrpc.DecreaseTargetSizeReply, error) {
	glog.Debugf("Call server DecreaseTargetSize: %v", request)

	if request.GetProviderID() != s.configuration.ProviderID {
		glog.Errorf(constantes.ErrMismatchingProvider)
		return nil, fmt.Errorf(constantes.ErrMismatchingProvider)
	}

	nodeGroup := s.Groups[request.GetNodeGroupID()]

	if nodeGroup == nil {
		glog.Errorf(constantes.ErrNodeGroupNotFound, request.GetNodeGroupID())

		return &apigrpc.DecreaseTargetSizeReply{
			Error: &apigrpc.Error{
				Code:   constantes.CloudProviderError,
				Reason: fmt.Sprintf(constantes.ErrNodeGroupNotFound, request.GetNodeGroupID()),
			},
		}, nil
	}

	if request.GetDelta() >= 0 {
		glog.Errorf(constantes.ErrDecreaseSizeMustBeNegative)

		return &apigrpc.DecreaseTargetSizeReply{
			Error: &apigrpc.Error{
				Code:   constantes.CloudProviderError,
				Reason: constantes.ErrDecreaseSizeMustBeNegative,
			},
		}, nil
	}

	newSize := nodeGroup.targetSize() + int(request.GetDelta())

	if newSize < len(nodeGroup.Nodes) {
		glog.Errorf(constantes.ErrDecreaseSizeAttemptDeleteNodes, nodeGroup.targetSize(), request.GetDelta(), newSize)

		return &apigrpc.DecreaseTargetSizeReply{
			Error: &apigrpc.Error{
				Code:   constantes.CloudProviderError,
				Reason: fmt.Sprintf(constantes.ErrDecreaseSizeAttemptDeleteNodes, nodeGroup.targetSize(), request.GetDelta(), newSize),
			},
		}, nil
	}

	err := nodeGroup.setNodeGroupSize(s.kubeClient, newSize)

	if err != nil {
		return &apigrpc.DecreaseTargetSizeReply{
			Error: &apigrpc.Error{
				Code:   constantes.CloudProviderError,
				Reason: err.Error(),
			},
		}, nil
	}

	return &apigrpc.DecreaseTargetSizeReply{
		Error: nil,
	}, nil
}

// Id returns an unique identifier of the node group.
func (s *AutoScalerServerApp) Id(ctx context.Context, request *apigrpc.NodeGroupServiceRequest) (*apigrpc.IdReply, error) {
	glog.Debugf("Call server Id: %v", request)

	if request.GetProviderID() != s.configuration.ProviderID {
		glog.Errorf(constantes.ErrMismatchingProvider)
		return nil, fmt.Errorf(constantes.ErrMismatchingProvider)
	}

	nodeGroup := s.Groups[request.GetNodeGroupID()]

	if nodeGroup == nil {
		glog.Errorf(constantes.ErrNodeGroupNotFound, request.GetNodeGroupID())
		return nil, fmt.Errorf(constantes.ErrNodeGroupNotFound, request.GetNodeGroupID())
	}

	return &apigrpc.IdReply{
		Response: nodeGroup.NodeGroupIdentifier,
	}, nil
}

// Debug returns a string containing all information regarding this node group.
func (s *AutoScalerServerApp) Debug(ctx context.Context, request *apigrpc.NodeGroupServiceRequest) (*apigrpc.DebugReply, error) {
	glog.Debugf("Call server Debug: %v", request)

	if request.GetProviderID() != s.configuration.ProviderID {
		glog.Errorf(constantes.ErrMismatchingProvider)
		return nil, fmt.Errorf(constantes.ErrMismatchingProvider)
	}

	nodeGroup := s.Groups[request.GetNodeGroupID()]

	if nodeGroup == nil {
		glog.Errorf(constantes.ErrNodeGroupNotFound, request.GetNodeGroupID())

		return nil, fmt.Errorf(constantes.ErrNodeGroupNotFound, request.GetNodeGroupID())
	}

	return &apigrpc.DebugReply{
		Response: fmt.Sprintf("%s-%s", request.GetProviderID(), nodeGroup.NodeGroupIdentifier),
	}, nil
}

// Nodes returns a list of all nodes that belong to this node group.
// It is required that Instance objects returned by this method have Id field set.
// Other fields are optional.
func (s *AutoScalerServerApp) Nodes(ctx context.Context, request *apigrpc.NodeGroupServiceRequest) (*apigrpc.NodesReply, error) {
	glog.Debugf("Call server Nodes: %v", request)

	if request.GetProviderID() != s.configuration.ProviderID {
		glog.Errorf(constantes.ErrMismatchingProvider)
		return nil, fmt.Errorf(constantes.ErrMismatchingProvider)
	}

	nodeGroup := s.Groups[request.GetNodeGroupID()]

	if nodeGroup == nil {
		glog.Errorf(constantes.ErrNodeGroupNotFound, request.GetNodeGroupID())

		return &apigrpc.NodesReply{
			Response: &apigrpc.NodesReply_Error{
				Error: &apigrpc.Error{
					Code:   constantes.CloudProviderError,
					Reason: fmt.Sprintf(constantes.ErrNodeGroupNotFound, request.GetNodeGroupID()),
				},
			},
		}, nil
	}

	instances := make([]*apigrpc.Instance, 0, len(nodeGroup.Nodes))

	for nodeName, node := range nodeGroup.Nodes {
		instances = append(instances, &apigrpc.Instance{
			Id: nodeGroup.providerIDForNode(nodeName),
			Status: &apigrpc.InstanceStatus{
				State:     apigrpc.InstanceState(node.State),
				ErrorInfo: nil,
			},
		})
	}

	return &apigrpc.NodesReply{
		Response: &apigrpc.NodesReply_Instances{
			Instances: &apigrpc.Instances{
				Items: instances,
			},
		},
	}, nil
}

// TemplateNodeInfo returns a schedulercache.NodeInfo structure of an empty
// (as if just started) node. This will be used in scale-up simulations to
// predict what would a new node look like if a node group was expanded. The returned
// NodeInfo is expected to have a fully populated Node object, with all of the labels,
// capacity and allocatable information as well as all pods that are started on
// the node by default, using manifest (most likely only kube-proxy). Implementation optional.
func (s *AutoScalerServerApp) TemplateNodeInfo(ctx context.Context, request *apigrpc.NodeGroupServiceRequest) (*apigrpc.TemplateNodeInfoReply, error) {
	glog.Debugf("Call server TemplateNodeInfo: %v", request)

	if s.configuration.Optionals.TemplateNodeInfo {
		return nil, fmt.Errorf(constantes.ErrNotImplemented)
	}

	if request.GetProviderID() != s.configuration.ProviderID {
		glog.Errorf(constantes.ErrMismatchingProvider)
		return nil, fmt.Errorf(constantes.ErrMismatchingProvider)
	}

	nodeGroup := s.Groups[request.GetNodeGroupID()]

	if nodeGroup == nil {
		glog.Errorf(constantes.ErrNodeGroupNotFound, request.GetNodeGroupID())

		return &apigrpc.TemplateNodeInfoReply{
			Response: &apigrpc.TemplateNodeInfoReply_Error{
				Error: &apigrpc.Error{
					Code:   constantes.CloudProviderError,
					Reason: fmt.Sprintf(constantes.ErrNodeGroupNotFound, request.GetNodeGroupID()),
				},
			},
		}, nil
	}

	node := &apiv1.Node{
		Spec: apiv1.NodeSpec{
			ProviderID:    nodeGroup.providerID(),
			Unschedulable: false,
		},
	}

	return &apigrpc.TemplateNodeInfoReply{
		Response: &apigrpc.TemplateNodeInfoReply_NodeInfo{NodeInfo: &apigrpc.NodeInfo{
			Node: utils.ToJSON(node),
		}},
	}, nil
}

// Exist checks if the node group really exists on the cloud provider side. Allows to tell the
// theoretical node group from the real one. Implementation required.
func (s *AutoScalerServerApp) Exist(ctx context.Context, request *apigrpc.NodeGroupServiceRequest) (*apigrpc.ExistReply, error) {
	glog.Debugf("Call server Exist: %v", request)

	if request.GetProviderID() != s.configuration.ProviderID {
		glog.Errorf(constantes.ErrMismatchingProvider)
		return nil, fmt.Errorf(constantes.ErrMismatchingProvider)
	}

	nodeGroup := s.Groups[request.GetNodeGroupID()]

	return &apigrpc.ExistReply{
		Exists: nodeGroup != nil,
	}, nil
}

// Create creates the node group on the cloud provider side. Implementation optional.
func (s *AutoScalerServerApp) Create(ctx context.Context, request *apigrpc.NodeGroupServiceRequest) (*apigrpc.CreateReply, error) {
	glog.Debugf("Call server Create: %v", request)

	if s.configuration.Optionals.Create {
		return nil, fmt.Errorf(constantes.ErrNotImplemented)
	}

	if request.GetProviderID() != s.configuration.ProviderID {
		glog.Errorf(constantes.ErrMismatchingProvider)
		return nil, fmt.Errorf(constantes.ErrMismatchingProvider)
	}

	nodeGroup, err := s.createNodeGroup(request.GetNodeGroupID())

	if err != nil {
		glog.Errorf(constantes.ErrUnableToCreateNodeGroup, request.GetNodeGroupID(), err)

		return &apigrpc.CreateReply{
			Response: &apigrpc.CreateReply_Error{
				Error: &apigrpc.Error{
					Code:   constantes.CloudProviderError,
					Reason: err.Error(),
				},
			},
		}, nil
	}

	return &apigrpc.CreateReply{
		Response: &apigrpc.CreateReply_NodeGroup{
			NodeGroup: &apigrpc.NodeGroup{
				Id: nodeGroup.NodeGroupIdentifier,
			},
		},
	}, nil
}

// Delete deletes the node group on the cloud provider side.
// This will be executed only for autoprovisioned node groups, once their size drops to 0.
// Implementation optional.
func (s *AutoScalerServerApp) Delete(ctx context.Context, request *apigrpc.NodeGroupServiceRequest) (*apigrpc.DeleteReply, error) {
	glog.Debugf("Call server Delete: %v", request)

	if s.configuration.Optionals.Delete {
		return nil, fmt.Errorf(constantes.ErrNotImplemented)
	}

	if request.GetProviderID() != s.configuration.ProviderID {
		glog.Errorf(constantes.ErrMismatchingProvider)
		return nil, fmt.Errorf(constantes.ErrMismatchingProvider)
	}

	err := s.deleteNodeGroup(request.GetNodeGroupID())

	if err != nil {
		glog.Errorf(constantes.ErrUnableToDeleteNodeGroup, request.GetNodeGroupID(), err)
		return &apigrpc.DeleteReply{
			Error: &apigrpc.Error{
				Code:   constantes.CloudProviderError,
				Reason: err.Error(),
			},
		}, nil
	}

	return &apigrpc.DeleteReply{
		Error: nil,
	}, nil
}

// Autoprovisioned returns true if the node group is autoprovisioned. An autoprovisioned group
// was created by CA and can be deleted when scaled to 0.
func (s *AutoScalerServerApp) Autoprovisioned(ctx context.Context, request *apigrpc.NodeGroupServiceRequest) (*apigrpc.AutoprovisionedReply, error) {
	glog.Debugf("Call server Autoprovisioned: %v", request)

	var b bool

	if request.GetProviderID() != s.configuration.ProviderID {
		glog.Errorf(constantes.ErrMismatchingProvider)
		return nil, fmt.Errorf(constantes.ErrMismatchingProvider)
	}

	ng := s.Groups[request.GetNodeGroupID()]

	if ng != nil {
		b = ng.AutoProvision
	}

	return &apigrpc.AutoprovisionedReply{
		Autoprovisioned: b,
	}, nil
}

// Belongs returns true if the given node belongs to the NodeGroup.
func (s *AutoScalerServerApp) Belongs(ctx context.Context, request *apigrpc.BelongsRequest) (*apigrpc.BelongsReply, error) {
	glog.Debugf("Call server Belongs: %v", request)

	if request.GetProviderID() != s.configuration.ProviderID {
		glog.Errorf(constantes.ErrMismatchingProvider)
		return nil, fmt.Errorf(constantes.ErrMismatchingProvider)
	}

	node, err := utils.NodeFromJSON(request.GetNode())

	if err != nil {
		glog.Errorf(constantes.ErrCantUnmarshallNodeWithReason, request.GetNode(), err)

		return &apigrpc.BelongsReply{
			Response: &apigrpc.BelongsReply_Error{
				Error: &apigrpc.Error{
					Code:   constantes.CloudProviderError,
					Reason: err.Error(),
				},
			},
		}, nil
	}

	providerID := utils.GetNodeProviderID(s.configuration.ProviderID, node)
	nodeGroup, _ := s.nodeGroupForNode(providerID)

	var belong bool

	if nodeGroup != nil {
		if nodeGroup.NodeGroupIdentifier == request.GetNodeGroupID() {
			nodeName, err := utils.NodeNameFromProviderID(s.configuration.ProviderID, providerID)

			if err != nil {
				return &apigrpc.BelongsReply{
					Response: &apigrpc.BelongsReply_Error{
						Error: &apigrpc.Error{
							Code:   constantes.CloudProviderError,
							Reason: err.Error(),
						},
					},
				}, nil
			}

			belong = nodeGroup.Nodes[nodeName] != nil
		}
	}

	return &apigrpc.BelongsReply{
		Response: &apigrpc.BelongsReply_Belongs{
			Belongs: belong,
		},
	}, nil
}

// NodePrice returns a price of running the given node for a given period of time.
// All prices returned by the structure should be in the same currency.
func (s *AutoScalerServerApp) NodePrice(ctx context.Context, request *apigrpc.NodePriceRequest) (*apigrpc.NodePriceReply, error) {
	glog.Debugf("Call server NodePrice: %v", request)

	if request.GetProviderID() != s.configuration.ProviderID {
		glog.Errorf(constantes.ErrMismatchingProvider)
		return nil, fmt.Errorf(constantes.ErrMismatchingProvider)
	}

	return &apigrpc.NodePriceReply{
		Response: &apigrpc.NodePriceReply_Price{
			Price: s.configuration.NodePrice,
		},
	}, nil
}

// PodPrice returns a theoretical minimum price of running a pod for a given
// period of time on a perfectly matching machine.
func (s *AutoScalerServerApp) PodPrice(ctx context.Context, request *apigrpc.PodPriceRequest) (*apigrpc.PodPriceReply, error) {
	glog.Debugf("Call server PodPrice: %v", request)

	if request.GetProviderID() != s.configuration.ProviderID {
		glog.Errorf(constantes.ErrMismatchingProvider)
		return nil, fmt.Errorf(constantes.ErrMismatchingProvider)
	}

	return &apigrpc.PodPriceReply{
		Response: &apigrpc.PodPriceReply_Price{
			Price: s.configuration.PodPrice,
		},
	}, nil
}

// Save state to file
func (s *AutoScalerServerApp) Save(fileName string) error {
	file, err := os.Create(fileName)

	if err != nil {
		glog.Errorf("Failed to open file:%s, error:%v", fileName, err)

		return err
	}

	defer file.Close()

	encoder := json.NewEncoder(file)
	err = encoder.Encode(s)

	if err != nil {
		glog.Errorf("failed to encode AutoScalerServerApp to file:%s, error:%v", fileName, err)

		return err
	}

	return nil
}

// Load saved state from file
func (s *AutoScalerServerApp) Load(fileName string) error {
	file, err := os.Open(fileName)

	if err != nil {
		glog.Errorf("Failed to open file:%s, error:%v", fileName, err)

		return err
	}

	defer file.Close()

	decoder := json.NewDecoder(file)
	err = decoder.Decode(s)

	if err != nil {
		glog.Errorf("failed to decode AutoScalerServerApp file:%s, error:%v", fileName, err)
		return err
	}

	for _, ng := range s.Groups {
		ng.setConfiguration(s.configuration)
	}

	if s.AutoProvision {
		if err := s.doAutoProvision(); err != nil {
			glog.Errorf(constantes.ErrUnableToAutoProvisionNodeGroup, err)

			return err
		}
	}

	return nil
}

func (s *AutoScalerServerApp) client() types.ClientGenerator {
	return s.kubeClient
}

func (s *AutoScalerServerApp) startController() error {
	var err error

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	if _, err = s.kubeClient.KubeClient(); err == nil {
		if _, err = s.kubeClient.NodeManagerClient(); err == nil {
			var controller *Controller

			if controller, err = NewController(s, stopCh); err == nil {
				err = controller.Run()
			} else {
				glog.Errorf("Create CRD failed, reason: %v", err)
			}
		} else {
			glog.Errorf("can't get manager node client interface, reason: %v", err)
		}
	} else {
		glog.Errorf("can't get kubeclient interface, reason: %v", err)
	}

	return err
}

func (s *AutoScalerServerApp) run(config *types.AutoScalerServerConfig) {
	lis, err := net.Listen(config.Network, config.Listen)

	if err != nil {
		glog.Fatalf("failed to listen: %v", err)
	}

	server := grpc.NewServer()

	defer func() {
		s.running = false
		server.Stop()
	}()

	apigrpc.RegisterCloudProviderServiceServer(server, s)
	apigrpc.RegisterNodeGroupServiceServer(server, s)
	apigrpc.RegisterPricingModelServiceServer(server, s)

	reflection.Register(server)

	if err := server.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}

	glog.Infof("End listening server")
}

// StartServer start the service
func StartServer(kubeClient types.ClientGenerator, c *types.Config) {
	var config types.AutoScalerServerConfig
	var autoScalerServer *AutoScalerServerApp

	saveState := c.SaveLocation
	configFileName := c.Config

	if len(saveState) > 0 {
		phSavedState = saveState
		phSaveState = true
	}

	file, err := os.Open(configFileName)
	if err != nil {
		glog.Fatalf("failed to open config file:%s, error:%v", configFileName, err)
	}

	decoder := json.NewDecoder(file)
	err = decoder.Decode(&config)
	if err != nil {
		glog.Fatalf("failed to decode config file:%s, error:%v", configFileName, err)
	}

	if _, err = kubeClient.KubeClient(); err != nil {
		glog.Fatalf("failed to get kubernetes client, error:%v", err)
	}

	if config.Optionals == nil {
		config.Optionals = &types.AutoScalerServerOptionals{
			Pricing:                  false,
			GetAvailableMachineTypes: false,
			NewNodeGroup:             false,
			TemplateNodeInfo:         false,
			Create:                   false,
			Delete:                   false,
		}
	}

	if config.UseExternalEtdc == nil {
		config.UseExternalEtdc = &c.UseExternalEtdc
	}

	if len(config.ExtDestinationEtcdSslDir) == 0 {
		config.ExtDestinationEtcdSslDir = c.ExtDestinationEtcdSslDir
	}

	if len(config.ExtSourceEtcdSslDir) == 0 {
		config.ExtSourceEtcdSslDir = c.ExtSourceEtcdSslDir
	}

	if len(config.KubernetesPKISourceDir) == 0 {
		config.KubernetesPKISourceDir = c.KubernetesPKISourceDir
	}

	if len(config.KubernetesPKIDestDir) == 0 {
		config.KubernetesPKIDestDir = c.KubernetesPKIDestDir
	}

	config.ManagedNodeResourceLimiter = c.GetManagedNodeResourceLimiter()

	if !phSaveState || !utils.FileExists(phSavedState) {
		autoScalerServer = &AutoScalerServerApp{
			kubeClient:      kubeClient,
			requestTimeout:  c.RequestTimeout,
			ResourceLimiter: c.GetResourceLimiter(),
			configuration:   &config,
			Groups:          make(map[string]*AutoScalerServerNodeGroup),
		}

		autoScalerServer.ResourceLimiter.SetMaxValue(constantes.ResourceNameNodes, config.MaxNode)
		autoScalerServer.ResourceLimiter.SetMinValue(constantes.ResourceNameNodes, config.MinNode)

		if phSaveState {
			if err = autoScalerServer.Save(phSavedState); err != nil {
				log.Fatalf(constantes.ErrFailedToSaveServerState, err)
			}
		}
	} else {
		autoScalerServer = &AutoScalerServerApp{
			kubeClient:     kubeClient,
			requestTimeout: c.RequestTimeout,
			configuration:  &config,
		}

		if err := autoScalerServer.Load(phSavedState); err != nil {
			log.Fatalf(constantes.ErrFailedToLoadServerState, err)
		}
	}

	if err = autoScalerServer.startController(); err != nil {
		glog.Fatalf("Can't start controller, reason:%s", err)
	}

	autoScalerServer.run(&config)
}
