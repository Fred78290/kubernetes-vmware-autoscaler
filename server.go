package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	apigrpc "github.com/Fred78290/kubernetes-vmware-autoscaler/grpc"
	"github.com/golang/glog"
	apiv1 "k8s.io/api/core/v1"
)

const (
	nodeLabelGroupName             = "cluster.autoscaler.nodegroup/name"
	annotationNodeIndex            = "cluster.autoscaler.nodegroup/node-index"
	annotationNodeAutoProvisionned = "cluster.autoscaler.nodegroup/autoprovision"
	annotationScaleDownDisabled    = "cluster-autoscaler.kubernetes.io/scale-down-disabled"
)

// ResourceLimiter define limit, not really used
type ResourceLimiter struct {
	MinLimits map[string]int64 `json:"min"`
	MaxLimits map[string]int64 `json:"max"`
}

// MachineCharacteristic defines VM kind
type MachineCharacteristic struct {
	Memory int `json:"memsize"`  // VM Memory size in megabytes
	Vcpu   int `json:"vcpus"`    // VM number of cpus
	Disk   int `json:"disksize"` // VM disk size in megabytes
}

// KubeJoinConfig give element to join kube master
type KubeJoinConfig struct {
	Address        string   `json:"address,omitempty"`
	Token          string   `json:"token,omitempty"`
	CACert         string   `json:"ca,omitempty"`
	ExtraArguments []string `json:"extras-args,omitempty"`
}

// AutoScalerServerOptionals declare wich features must be optional
type AutoScalerServerOptionals struct {
	Pricing                  bool `json:"pricing"`
	GetAvailableMachineTypes bool `json:"getAvailableMachineTypes"`
	NewNodeGroup             bool `json:"newNodeGroup"`
	TemplateNodeInfo         bool `json:"templateNodeInfo"`
	Create                   bool `json:"create"`
	Delete                   bool `json:"delete"`
}

// AutoScalerServerSSH contains ssh client infos
type AutoScalerServerSSH struct {
	UserName string
	Password string
	AuthKeys string
}

// VSphereConfig declares vsphere connection info
type VSphereConfig struct {
	Host       string `json:"host"`
	UserName   string `json:"uid"`
	Password   string `json:"password"`
	Insecure   bool   `json:"insecure"`
	DataCenter string `json:"dc"`
	DataStore  string `json:"datastore"`
	Resource   string `json:"resource"`
	VMPath     string `json:"vm"`
}

// AutoScalerServerConfig is contains configuration
type AutoScalerServerConfig struct {
	Network            string                            `default:"tcp" json:"network"`         // Mandatory, Network to listen (see grpc doc) to listen
	Listen             string                            `default:"0.0.0.0:5200" json:"listen"` // Mandatory, Address to listen
	ProviderID         string                            `json:"secret"`                        // Mandatory, secret Identifier, client must match this
	MinNode            int                               `json:"minNode"`                       // Mandatory, Min AutoScaler VM
	MaxNode            int                               `json:"maxNode"`                       // Mandatory, Max AutoScaler VM
	NodePrice          float64                           `json:"nodePrice"`                     // Optional, The VM price
	PodPrice           float64                           `json:"podPrice"`                      // Optional, The pod price
	Image              string                            `json:"image"`                         // Mandatory, VM to clone
	KubeCtlConfig      string                            `default:"/etc/kubernetes/config" json:"kubeconfig"`
	KubeAdm            KubeJoinConfig                    `json:"kubeadm"`
	DefaultMachineType string                            `default:"{\"standard\": {}}" json:"default-machine"`
	Machines           map[string]*MachineCharacteristic `default:"{\"standard\": {}}" json:"machines"` // Mandatory, Available machines
	CloudInit          map[string]interface{}            `json:"cloud-init"`                            // Optional, The cloud init conf file
	MountPoints        map[string]string                 `json:"mount-points"`                          // Optional, mount point between host and guest
	VMProvision        bool                              `default:"true" json:"vm-provision"`
	Optionals          *AutoScalerServerOptionals        `json:"optionals"`
	SSH                *AutoScalerServerSSH              `json:"ssh-infos"`
	VSphere            VSphereConfig                     `json:"vsphere-infos"`
}

// AutoScalerServerApp declare AutoScaler grpc server
type AutoScalerServerApp struct {
	ResourceLimiter      *ResourceLimiter                      `json:"limits"`
	Groups               map[string]*AutoScalerServerNodeGroup `json:"groups"`
	Configuration        AutoScalerServerConfig                `json:"config"`
	KubeAdmConfiguration *apigrpc.KubeAdmConfig                `json:"kubeadm"`
	NodesDefinition      []*apigrpc.NodeGroupDef               `json:"nodedefs"`
	AutoProvision        bool                                  `json:"auto"`
}

func (s *AutoScalerServerApp) generateNodeGroupName() string {
	return fmt.Sprintf("ng-%d", time.Now().Unix())
}

func (s *AutoScalerServerApp) newNodeGroup(nodeGroupID string, minNodeSize, maxNodeSize int32, machineType string, labels, systemLabels map[string]string, autoProvision bool) (*AutoScalerServerNodeGroup, error) {

	machine := s.Configuration.Machines[machineType]

	if machine == nil {
		return nil, fmt.Errorf(errMachineTypeNotFound, machineType)
	}

	if nodeGroup := s.Groups[nodeGroupID]; nodeGroup != nil {
		glog.Errorf(errNodeGroupAlreadyExists, nodeGroupID)

		return nil, fmt.Errorf(errNodeGroupAlreadyExists, nodeGroupID)
	}

	glog.Infof("New node group, ID:%s minSize:%d, maxSize:%d, machineType:%s, node lables:%v, %v", nodeGroupID, minNodeSize, maxNodeSize, machineType, labels, systemLabels)

	nodeGroup := &AutoScalerServerNodeGroup{
		ServiceIdentifier:   s.Configuration.ProviderID,
		NodeGroupIdentifier: nodeGroupID,
		Machine:             machine,
		Status:              NodegroupNotCreated,
		PendingNodes:        make(map[string]*AutoScalerServerNode),
		Nodes:               make(map[string]*AutoScalerServerNode),
		MinNodeSize:         int(minNodeSize),
		MaxNodeSize:         int(maxNodeSize),
		NodeLabels:          labels,
		SystemLabels:        systemLabels,
		AutoProvision:       autoProvision,
	}

	s.Groups[nodeGroupID] = nodeGroup

	return nodeGroup, nil
}

func (s *AutoScalerServerApp) deleteNodeGroup(nodeGroupID string) error {
	nodeGroup := s.Groups[nodeGroupID]

	if nodeGroup == nil {
		glog.Errorf(errNodeGroupNotFound, nodeGroupID)
		return fmt.Errorf(errNodeGroupNotFound, nodeGroupID)
	}

	glog.Infof("Delete node group, ID:%s", nodeGroupID)

	if err := nodeGroup.deleteNodeGroup(s.Configuration.KubeCtlConfig); err != nil {
		glog.Errorf(errUnableToDeleteNodeGroup, nodeGroupID, err)
		return err
	}

	delete(s.Groups, nodeGroupID)

	return nil
}

func (s *AutoScalerServerApp) createNodeGroup(nodeGroupID string) (*AutoScalerServerNodeGroup, error) {
	nodeGroup := s.Groups[nodeGroupID]

	if nodeGroup == nil {
		glog.Errorf(errNodeGroupNotFound, nodeGroupID)
		return nil, fmt.Errorf(errNodeGroupNotFound, nodeGroupID)
	}

	if nodeGroup.Status == NodegroupNotCreated {
		// Must launch minNode VM
		if nodeGroup.MinNodeSize > 0 {

			glog.Infof("Create node group, ID:%s", nodeGroupID)

			extras := &nodeCreationExtra{
				kubeHost:      s.KubeAdmConfiguration.KubeAdmAddress,
				kubeToken:     s.KubeAdmConfiguration.KubeAdmToken,
				kubeCACert:    s.KubeAdmConfiguration.KubeAdmCACert,
				kubeExtraArgs: s.KubeAdmConfiguration.KubeAdmExtraArguments,
				kubeConfig:    s.Configuration.KubeCtlConfig,
				image:         s.Configuration.Image,
				cloudInit:     s.Configuration.CloudInit,
				mountPoints:   s.Configuration.MountPoints,
				nodegroupID:   nodeGroupID,
				nodeLabels:    nodeGroup.NodeLabels,
				systemLabels:  nodeGroup.SystemLabels,
				vmprovision:   s.Configuration.VMProvision,
			}

			if err := nodeGroup.addNodes(nodeGroup.MinNodeSize, extras); err != nil {
				glog.Errorf(err.Error())

				return nil, err
			}
		}

		nodeGroup.Status = NodegroupCreated
	}

	return nodeGroup, nil
}

func (s *AutoScalerServerApp) doAutoProvision() error {
	glog.V(5).Info("Call server doAutoProvision")

	var ng *AutoScalerServerNodeGroup
	var err error

	for _, node := range s.NodesDefinition {
		nodeGroupIdentifier := node.GetNodeGroupID()

		if len(nodeGroupIdentifier) > 0 {
			ng = s.Groups[nodeGroupIdentifier]

			if ng == nil {
				systemLabels := make(map[string]string)
				labels := map[string]string{
					nodeLabelGroupName: nodeGroupIdentifier,
				}

				// Default labels
				if node.GetLabels() != nil {
					for k, v := range node.GetLabels() {
						labels[k] = v
					}
				}

				glog.Infof("Auto provision for nodegroup:%s, minSize:%d, maxSize:%d", nodeGroupIdentifier, node.MinSize, node.MaxSize)

				if ng, err = s.newNodeGroup(nodeGroupIdentifier, node.MinSize, node.MaxSize, s.Configuration.DefaultMachineType, labels, systemLabels, true); err == nil {
					if ng, err = s.createNodeGroup(nodeGroupIdentifier); err == nil {
						if node.GetIncludeExistingNode() {
							if err = ng.autoDiscoveryNodes(true, s.Configuration.KubeCtlConfig); err == nil {
								return err
							}
						}
					}
				}

				if err != nil {
					break
				}
			} else {
				// If the nodegroup already exists, reparse nodes
				if node.GetIncludeExistingNode() {
					if err = ng.autoDiscoveryNodes(true, s.Configuration.KubeCtlConfig); err == nil {
						return err
					}
				}
			}
		}
	}

	return err
}

// Connect allows client to connect
func (s *AutoScalerServerApp) Connect(ctx context.Context, request *apigrpc.ConnectRequest) (*apigrpc.ConnectReply, error) {
	glog.V(5).Infof("Call server Connect: %v", request)

	if request.GetProviderID() != s.Configuration.ProviderID {
		glog.Errorf(errMismatchingProvider)
		return nil, fmt.Errorf(errMismatchingProvider)
	}

	if request.GetResourceLimiter() != nil {
		s.ResourceLimiter = &ResourceLimiter{
			MinLimits: request.ResourceLimiter.MinLimits,
			MaxLimits: request.ResourceLimiter.MaxLimits,
		}
	}

	s.NodesDefinition = request.GetNodes()
	s.AutoProvision = request.GetAutoProvisionned()

	if request.GetKubeAdmConfiguration() != nil {
		s.KubeAdmConfiguration = request.GetKubeAdmConfiguration()
	}

	if s.AutoProvision {
		if err := s.doAutoProvision(); err != nil {
			glog.Errorf(errUnableToAutoProvisionNodeGroup, err)

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
	glog.V(5).Infof("Call server Name: %v", request)

	if request.GetProviderID() != s.Configuration.ProviderID {
		glog.Errorf(errMismatchingProvider)
		return nil, fmt.Errorf(errMismatchingProvider)
	}

	return &apigrpc.NameReply{
		Name: providerName,
	}, nil
}

// NodeGroups returns all node groups configured for this cloud provider.
func (s *AutoScalerServerApp) NodeGroups(ctx context.Context, request *apigrpc.CloudProviderServiceRequest) (*apigrpc.NodeGroupsReply, error) {
	glog.V(5).Infof("Call server NodeGroups: %v", request)

	if request.GetProviderID() != s.Configuration.ProviderID {
		glog.Errorf(errMismatchingProvider)
		return nil, fmt.Errorf(errMismatchingProvider)
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
	nodeGroupID, err := nodeGroupIDFromProviderID(s.Configuration.ProviderID, providerID)

	if err != nil {
		glog.Errorf(errCantDecodeNodeIDWithReason, providerID, err)

		return nil, fmt.Errorf(errCantDecodeNodeIDWithReason, providerID, err)
	}

	if len(nodeGroupID) == 0 {
		glog.Errorf(errCantDecodeNodeID, providerID)

		return nil, fmt.Errorf(errCantDecodeNodeID, providerID)
	}

	nodeGroup := s.Groups[nodeGroupID]

	if nodeGroup == nil {
		glog.Errorf(errNodeGroupForNodeNotFound, nodeGroupID, providerID)

		//return nil, fmt.Errorf(errNodeGroupForNodeNotFound, nodeGroupID, providerID)
	}

	return nodeGroup, err
}

// NodeGroupForNode returns the node group for the given node, nil if the node
// should not be processed by cluster autoscaler, or non-nil error if such
// occurred. Must be implemented.
func (s *AutoScalerServerApp) NodeGroupForNode(ctx context.Context, request *apigrpc.NodeGroupForNodeRequest) (*apigrpc.NodeGroupForNodeReply, error) {
	glog.V(5).Infof("Call server NodeGroupForNode: %v", request)

	if request.GetProviderID() != s.Configuration.ProviderID {
		glog.Errorf(errMismatchingProvider)
		return nil, fmt.Errorf(errMismatchingProvider)
	}

	node, err := nodeFromJSON(request.GetNode())

	if err != nil {
		glog.Errorf(errCantUnmarshallNodeWithReason, request.GetNode(), err)

		return &apigrpc.NodeGroupForNodeReply{
			Response: &apigrpc.NodeGroupForNodeReply_Error{
				Error: &apigrpc.Error{
					Code:   cloudProviderError,
					Reason: err.Error(),
				},
			},
		}, nil
	}

	providerID := getNodeProviderID(s.Configuration.ProviderID, node)

	if len(providerID) == 0 {
		glog.V(5).Info("node.Spec.ProviderID is empty")
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
					Code:   cloudProviderError,
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
	glog.V(5).Infof("Call server Pricing: %v", request)

	if s.Configuration.Optionals.Pricing {
		return nil, fmt.Errorf(errNotImplemented)
	}

	if request.GetProviderID() != s.Configuration.ProviderID {
		glog.Errorf(errMismatchingProvider)
		return nil, fmt.Errorf(errMismatchingProvider)
	}

	return &apigrpc.PricingModelReply{
		Response: &apigrpc.PricingModelReply_PriceModel{
			PriceModel: &apigrpc.PricingModel{
				Id: s.Configuration.ProviderID,
			},
		},
	}, nil
}

// GetAvailableMachineTypes get all machine types that can be requested from the cloud provider.
// Implementation optional.
func (s *AutoScalerServerApp) GetAvailableMachineTypes(ctx context.Context, request *apigrpc.CloudProviderServiceRequest) (*apigrpc.AvailableMachineTypesReply, error) {
	glog.V(5).Infof("Call server GetAvailableMachineTypes: %v", request)

	if s.Configuration.Optionals.GetAvailableMachineTypes {
		return nil, fmt.Errorf(errNotImplemented)
	}

	if request.GetProviderID() != s.Configuration.ProviderID {
		glog.Errorf(errMismatchingProvider)
		return nil, fmt.Errorf(errMismatchingProvider)
	}

	machineTypes := make([]string, 0, len(s.Configuration.Machines))

	for n := range s.Configuration.Machines {
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
	glog.V(5).Infof("Call server NewNodeGroup: %v", request)

	if s.Configuration.Optionals.NewNodeGroup {
		return nil, fmt.Errorf(errNotImplemented)
	}

	if request.GetProviderID() != s.Configuration.ProviderID {
		glog.Errorf(errMismatchingProvider)
		return nil, fmt.Errorf(errMismatchingProvider)
	}

	machineType := s.Configuration.Machines[request.GetMachineType()]

	if machineType == nil {
		glog.Errorf(errMachineTypeNotFound, request.GetMachineType())

		return &apigrpc.NewNodeGroupReply{
			Response: &apigrpc.NewNodeGroupReply_Error{
				Error: &apigrpc.Error{
					Code:   cloudProviderError,
					Reason: fmt.Sprintf(errMachineTypeNotFound, request.GetMachineType()),
				},
			},
		}, nil
	}

	var nodeGroupIdentifier string

	labels := make(map[string]string)
	systemLabels := make(map[string]string)

	if request.GetLabels() != nil {
		for k2, v2 := range request.GetLabels() {
			labels[k2] = v2
		}
	}

	if request.GetSystemLabels() != nil {
		for k2, v2 := range request.GetSystemLabels() {
			systemLabels[k2] = v2
		}
	}

	if len(request.GetNodeGroupID()) == 0 {
		nodeGroupIdentifier = s.generateNodeGroupName()
	} else {
		nodeGroupIdentifier = request.GetNodeGroupID()
	}

	labels[nodeLabelGroupName] = nodeGroupIdentifier

	nodeGroup, err := s.newNodeGroup(nodeGroupIdentifier, request.GetMinNodeSize(), request.GetMaxNodeSize(), request.GetMachineType(), labels, systemLabels, false)

	if err != nil {
		glog.Errorf(errUnableToCreateNodeGroup, nodeGroupIdentifier, err)

		return &apigrpc.NewNodeGroupReply{
			Response: &apigrpc.NewNodeGroupReply_Error{
				Error: &apigrpc.Error{
					Code:   cloudProviderError,
					Reason: fmt.Sprintf(errUnableToCreateNodeGroup, nodeGroupIdentifier, err),
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
	glog.V(5).Infof("Call server GetResourceLimiter: %v", request)

	if request.GetProviderID() != s.Configuration.ProviderID {
		glog.Errorf(errMismatchingProvider)
		return nil, fmt.Errorf(errMismatchingProvider)
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

// Cleanup cleans up open resources before the cloud provider is destroyed, i.e. go routines etc.
func (s *AutoScalerServerApp) Cleanup(ctx context.Context, request *apigrpc.CloudProviderServiceRequest) (*apigrpc.CleanupReply, error) {
	glog.V(5).Infof("Call server Cleanup: %v", request)

	var lastError *apigrpc.Error

	if request.GetProviderID() != s.Configuration.ProviderID {
		glog.Errorf(errMismatchingProvider)
		return nil, fmt.Errorf(errMismatchingProvider)
	}

	for _, nodeGroup := range s.Groups {
		if err := nodeGroup.cleanup(s.Configuration.KubeCtlConfig); err != nil {
			lastError = &apigrpc.Error{
				Code:   cloudProviderError,
				Reason: err.Error(),
			}
		}
	}

	glog.V(5).Info("Leave server Cleanup, done")

	s.Groups = make(map[string]*AutoScalerServerNodeGroup)

	return &apigrpc.CleanupReply{
		Error: lastError,
	}, nil
}

// Refresh is called before every main loop and can be used to dynamically update cloud provider state.
// In particular the list of node groups returned by NodeGroups can change as a result of CloudProvider.Refresh().
func (s *AutoScalerServerApp) Refresh(ctx context.Context, request *apigrpc.CloudProviderServiceRequest) (*apigrpc.RefreshReply, error) {
	glog.V(5).Infof("Call server Refresh: %v", request)

	if request.GetProviderID() != s.Configuration.ProviderID {
		glog.Errorf(errMismatchingProvider)
		return nil, fmt.Errorf(errMismatchingProvider)
	}

	for _, ng := range s.Groups {
		ng.refresh()
	}

	if phSaveState {
		if err := s.save(phSavedState); err != nil {
			glog.Errorf(errFailedToSaveServerState, err)
		}
	}

	return &apigrpc.RefreshReply{
		Error: nil,
	}, nil
}

// MaxSize returns maximum size of the node group.
func (s *AutoScalerServerApp) MaxSize(ctx context.Context, request *apigrpc.NodeGroupServiceRequest) (*apigrpc.MaxSizeReply, error) {
	glog.V(5).Infof("Call server MaxSize: %v", request)

	var maxSize int

	if request.GetProviderID() != s.Configuration.ProviderID {
		glog.Errorf(errMismatchingProvider)
		return nil, fmt.Errorf(errMismatchingProvider)
	}

	nodeGroup := s.Groups[request.GetNodeGroupID()]

	if nodeGroup == nil {
		glog.Errorf(errNodeGroupNotFound, request.GetNodeGroupID())
	} else {
		maxSize = nodeGroup.MaxNodeSize
	}

	return &apigrpc.MaxSizeReply{
		MaxSize: int32(maxSize),
	}, nil
}

// MinSize returns minimum size of the node group.
func (s *AutoScalerServerApp) MinSize(ctx context.Context, request *apigrpc.NodeGroupServiceRequest) (*apigrpc.MinSizeReply, error) {
	glog.V(5).Infof("Call server MinSize: %v", request)

	var minSize int

	if request.GetProviderID() != s.Configuration.ProviderID {
		return nil, fmt.Errorf(errMismatchingProvider)
	}

	nodeGroup := s.Groups[request.GetNodeGroupID()]

	if nodeGroup == nil {
		glog.Errorf(errNodeGroupNotFound, request.GetNodeGroupID())
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
	glog.V(5).Infof("Call server TargetSize: %v", request)

	if request.GetProviderID() != s.Configuration.ProviderID {
		glog.Errorf(errMismatchingProvider)
		return nil, fmt.Errorf(errMismatchingProvider)
	}

	nodeGroup := s.Groups[request.GetNodeGroupID()]

	if nodeGroup == nil {
		glog.Errorf(errNodeGroupNotFound, request.GetNodeGroupID())

		return &apigrpc.TargetSizeReply{
			Response: &apigrpc.TargetSizeReply_Error{
				Error: &apigrpc.Error{
					Code:   cloudProviderError,
					Reason: fmt.Sprintf(errNodeGroupNotFound, request.GetNodeGroupID()),
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
	glog.V(5).Infof("Call server IncreaseSize: %v", request)

	if request.GetProviderID() != s.Configuration.ProviderID {
		glog.Errorf(errMismatchingProvider)
		return nil, fmt.Errorf(errMismatchingProvider)
	}

	nodeGroup := s.Groups[request.GetNodeGroupID()]

	if nodeGroup == nil {
		glog.Errorf(errNodeGroupNotFound, request.GetNodeGroupID())

		return &apigrpc.IncreaseSizeReply{
			Error: &apigrpc.Error{
				Code:   cloudProviderError,
				Reason: fmt.Sprintf(errNodeGroupNotFound, request.GetNodeGroupID()),
			},
		}, nil
	}

	if request.GetDelta() <= 0 {
		glog.Errorf(errIncreaseSizeMustBePositive)

		return &apigrpc.IncreaseSizeReply{
			Error: &apigrpc.Error{
				Code:   cloudProviderError,
				Reason: errIncreaseSizeMustBePositive,
			},
		}, nil
	}

	newSize := len(nodeGroup.Nodes) + int(request.GetDelta())

	if newSize > nodeGroup.MaxNodeSize {
		glog.Errorf(errIncreaseSizeTooLarge, newSize, nodeGroup.MaxNodeSize)

		return &apigrpc.IncreaseSizeReply{
			Error: &apigrpc.Error{
				Code:   cloudProviderError,
				Reason: fmt.Sprintf(errIncreaseSizeTooLarge, newSize, nodeGroup.MaxNodeSize),
			},
		}, nil
	}

	extras := &nodeCreationExtra{
		kubeHost:      s.KubeAdmConfiguration.KubeAdmAddress,
		kubeToken:     s.KubeAdmConfiguration.KubeAdmToken,
		kubeCACert:    s.KubeAdmConfiguration.KubeAdmCACert,
		kubeExtraArgs: s.KubeAdmConfiguration.KubeAdmExtraArguments,
		kubeConfig:    s.Configuration.KubeCtlConfig,
		image:         s.Configuration.Image,
		cloudInit:     s.Configuration.CloudInit,
		mountPoints:   s.Configuration.MountPoints,
		nodegroupID:   nodeGroup.NodeGroupIdentifier,
		nodeLabels:    nodeGroup.NodeLabels,
		systemLabels:  nodeGroup.SystemLabels,
		vmprovision:   s.Configuration.VMProvision,
	}

	err := nodeGroup.setNodeGroupSize(newSize, extras)

	if err != nil {
		return &apigrpc.IncreaseSizeReply{
			Error: &apigrpc.Error{
				Code:   cloudProviderError,
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
	glog.V(5).Infof("Call server DeleteNodes: %v", request)

	if request.GetProviderID() != s.Configuration.ProviderID {
		glog.Errorf(errMismatchingProvider)
		return nil, fmt.Errorf(errMismatchingProvider)
	}

	nodeGroup := s.Groups[request.GetNodeGroupID()]

	if nodeGroup == nil {
		glog.Errorf(errNodeGroupNotFound, request.GetNodeGroupID())

		return &apigrpc.DeleteNodesReply{
			Error: &apigrpc.Error{
				Code:   cloudProviderError,
				Reason: fmt.Sprintf(errNodeGroupNotFound, request.GetNodeGroupID()),
			},
		}, nil
	}

	if nodeGroup.targetSize()-len(request.GetNode()) < nodeGroup.MinNodeSize {
		return &apigrpc.DeleteNodesReply{
			Error: &apigrpc.Error{
				Code:   cloudProviderError,
				Reason: fmt.Sprintf(errMinSizeReached, request.GetNodeGroupID()),
			},
		}, nil
	}

	// Iterate over each requested node to delete
	for idx, sNode := range request.GetNode() {
		node, err := nodeFromJSON(sNode)

		// Can't deserialize
		if node == nil || err != nil {
			glog.Errorf(errCantUnmarshallNodeWithReason, sNode, err)

			return &apigrpc.DeleteNodesReply{
				Error: &apigrpc.Error{
					Code:   cloudProviderError,
					Reason: fmt.Sprintf(errCantUnmarshallNode, idx, request.GetNodeGroupID()),
				},
			}, nil
		}

		// Check node group owner
		nodeName := getNodeProviderID(s.Configuration.ProviderID, node)
		nodeGroupForNode, err := s.nodeGroupForNode(nodeName)

		// Node group not found
		if err != nil {
			glog.Errorf(errNodeGroupNotFound, nodeName)

			return &apigrpc.DeleteNodesReply{
				Error: &apigrpc.Error{
					Code:   cloudProviderError,
					Reason: err.Error(),
				},
			}, nil
		}

		// Not in the same group
		if nodeGroupForNode.NodeGroupIdentifier != nodeGroup.NodeGroupIdentifier {
			return &apigrpc.DeleteNodesReply{
				Error: &apigrpc.Error{
					Code:   cloudProviderError,
					Reason: fmt.Sprintf(errUnableToDeleteNode, nodeName, nodeGroup.NodeGroupIdentifier),
				},
			}, nil
		}

		// Delete the node in the group
		nodeName, err = nodeNameFromProviderID(s.Configuration.ProviderID, nodeName)

		err = nodeGroup.deleteNodeByName(s.Configuration.KubeCtlConfig, nodeName)

		if err != nil {
			return &apigrpc.DeleteNodesReply{
				Error: &apigrpc.Error{
					Code:   cloudProviderError,
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
	glog.V(5).Infof("Call server DecreaseTargetSize: %v", request)

	if request.GetProviderID() != s.Configuration.ProviderID {
		glog.Errorf(errMismatchingProvider)
		return nil, fmt.Errorf(errMismatchingProvider)
	}

	nodeGroup := s.Groups[request.GetNodeGroupID()]

	if nodeGroup == nil {
		glog.Errorf(errNodeGroupNotFound, request.GetNodeGroupID())

		return &apigrpc.DecreaseTargetSizeReply{
			Error: &apigrpc.Error{
				Code:   cloudProviderError,
				Reason: fmt.Sprintf(errNodeGroupNotFound, request.GetNodeGroupID()),
			},
		}, nil
	}

	if request.GetDelta() >= 0 {
		glog.Errorf(errDecreaseSizeMustBeNegative)

		return &apigrpc.DecreaseTargetSizeReply{
			Error: &apigrpc.Error{
				Code:   cloudProviderError,
				Reason: errDecreaseSizeMustBeNegative,
			},
		}, nil
	}

	newSize := nodeGroup.targetSize() + int(request.GetDelta())

	if newSize < len(nodeGroup.Nodes) {
		glog.Errorf(errDecreaseSizeAttemptDeleteNodes, nodeGroup.targetSize(), request.GetDelta(), newSize)

		return &apigrpc.DecreaseTargetSizeReply{
			Error: &apigrpc.Error{
				Code:   cloudProviderError,
				Reason: fmt.Sprintf(errDecreaseSizeAttemptDeleteNodes, nodeGroup.targetSize(), request.GetDelta(), newSize),
			},
		}, nil
	}

	extras := &nodeCreationExtra{
		kubeHost:      s.KubeAdmConfiguration.KubeAdmAddress,
		kubeToken:     s.KubeAdmConfiguration.KubeAdmToken,
		kubeCACert:    s.KubeAdmConfiguration.KubeAdmCACert,
		kubeExtraArgs: s.KubeAdmConfiguration.KubeAdmExtraArguments,
		kubeConfig:    s.Configuration.KubeCtlConfig,
		image:         s.Configuration.Image,
		cloudInit:     s.Configuration.CloudInit,
		mountPoints:   s.Configuration.MountPoints,
		nodegroupID:   nodeGroup.NodeGroupIdentifier,
		nodeLabels:    nodeGroup.NodeLabels,
		systemLabels:  nodeGroup.SystemLabels,
		vmprovision:   s.Configuration.VMProvision,
	}

	err := nodeGroup.setNodeGroupSize(newSize, extras)

	if err != nil {
		return &apigrpc.DecreaseTargetSizeReply{
			Error: &apigrpc.Error{
				Code:   cloudProviderError,
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
	glog.V(5).Infof("Call server Id: %v", request)

	if request.GetProviderID() != s.Configuration.ProviderID {
		glog.Errorf(errMismatchingProvider)
		return nil, fmt.Errorf(errMismatchingProvider)
	}

	nodeGroup := s.Groups[request.GetNodeGroupID()]

	if nodeGroup == nil {
		glog.Errorf(errNodeGroupNotFound, request.GetNodeGroupID())

		return nil, fmt.Errorf(errNodeGroupNotFound, request.GetNodeGroupID())
	}

	return &apigrpc.IdReply{
		Response: nodeGroup.NodeGroupIdentifier,
	}, nil
}

// Debug returns a string containing all information regarding this node group.
func (s *AutoScalerServerApp) Debug(ctx context.Context, request *apigrpc.NodeGroupServiceRequest) (*apigrpc.DebugReply, error) {
	glog.V(5).Infof("Call server Debug: %v", request)

	if request.GetProviderID() != s.Configuration.ProviderID {
		glog.Errorf(errMismatchingProvider)
		return nil, fmt.Errorf(errMismatchingProvider)
	}

	nodeGroup := s.Groups[request.GetNodeGroupID()]

	if nodeGroup == nil {
		glog.Errorf(errNodeGroupNotFound, request.GetNodeGroupID())

		return nil, fmt.Errorf(errNodeGroupNotFound, request.GetNodeGroupID())
	}

	return &apigrpc.DebugReply{
		Response: fmt.Sprintf("%s-%s", request.GetProviderID(), nodeGroup.NodeGroupIdentifier),
	}, nil
}

// Nodes returns a list of all nodes that belong to this node group.
// It is required that Instance objects returned by this method have Id field set.
// Other fields are optional.
func (s *AutoScalerServerApp) Nodes(ctx context.Context, request *apigrpc.NodeGroupServiceRequest) (*apigrpc.NodesReply, error) {
	glog.V(5).Infof("Call server Nodes: %v", request)

	if request.GetProviderID() != s.Configuration.ProviderID {
		glog.Errorf(errMismatchingProvider)
		return nil, fmt.Errorf(errMismatchingProvider)
	}

	nodeGroup := s.Groups[request.GetNodeGroupID()]

	if nodeGroup == nil {
		glog.Errorf(errNodeGroupNotFound, request.GetNodeGroupID())

		return &apigrpc.NodesReply{
			Response: &apigrpc.NodesReply_Error{
				Error: &apigrpc.Error{
					Code:   cloudProviderError,
					Reason: fmt.Sprintf(errNodeGroupNotFound, request.GetNodeGroupID()),
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
	glog.V(5).Infof("Call server TemplateNodeInfo: %v", request)

	if s.Configuration.Optionals.TemplateNodeInfo {
		return nil, fmt.Errorf(errNotImplemented)
	}

	if request.GetProviderID() != s.Configuration.ProviderID {
		glog.Errorf(errMismatchingProvider)
		return nil, fmt.Errorf(errMismatchingProvider)
	}

	nodeGroup := s.Groups[request.GetNodeGroupID()]

	if nodeGroup == nil {
		glog.Errorf(errNodeGroupNotFound, request.GetNodeGroupID())

		return &apigrpc.TemplateNodeInfoReply{
			Response: &apigrpc.TemplateNodeInfoReply_Error{
				Error: &apigrpc.Error{
					Code:   cloudProviderError,
					Reason: fmt.Sprintf(errNodeGroupNotFound, request.GetNodeGroupID()),
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
			Node: toJSON(node),
		}},
	}, nil
}

// Exist checks if the node group really exists on the cloud provider side. Allows to tell the
// theoretical node group from the real one. Implementation required.
func (s *AutoScalerServerApp) Exist(ctx context.Context, request *apigrpc.NodeGroupServiceRequest) (*apigrpc.ExistReply, error) {
	glog.V(5).Infof("Call server Exist: %v", request)

	if request.GetProviderID() != s.Configuration.ProviderID {
		glog.Errorf(errMismatchingProvider)
		return nil, fmt.Errorf(errMismatchingProvider)
	}

	nodeGroup := s.Groups[request.GetNodeGroupID()]

	return &apigrpc.ExistReply{
		Exists: nodeGroup != nil,
	}, nil
}

// Create creates the node group on the cloud provider side. Implementation optional.
func (s *AutoScalerServerApp) Create(ctx context.Context, request *apigrpc.NodeGroupServiceRequest) (*apigrpc.CreateReply, error) {
	glog.V(5).Infof("Call server Create: %v", request)

	if s.Configuration.Optionals.Create {
		return nil, fmt.Errorf(errNotImplemented)
	}

	if request.GetProviderID() != s.Configuration.ProviderID {
		glog.Errorf(errMismatchingProvider)
		return nil, fmt.Errorf(errMismatchingProvider)
	}

	nodeGroup, err := s.createNodeGroup(request.GetNodeGroupID())

	if err != nil {
		glog.Errorf(errUnableToCreateNodeGroup, request.GetNodeGroupID(), err)

		return &apigrpc.CreateReply{
			Response: &apigrpc.CreateReply_Error{
				Error: &apigrpc.Error{
					Code:   cloudProviderError,
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
	glog.V(5).Infof("Call server Delete: %v", request)

	if s.Configuration.Optionals.Delete {
		return nil, fmt.Errorf(errNotImplemented)
	}

	if request.GetProviderID() != s.Configuration.ProviderID {
		glog.Errorf(errMismatchingProvider)
		return nil, fmt.Errorf(errMismatchingProvider)
	}

	err := s.deleteNodeGroup(request.GetNodeGroupID())

	if err != nil {
		glog.Errorf(errUnableToDeleteNodeGroup, request.GetNodeGroupID(), err)
		return &apigrpc.DeleteReply{
			Error: &apigrpc.Error{
				Code:   cloudProviderError,
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
	glog.V(5).Infof("Call server Autoprovisioned: %v", request)

	var b bool

	if request.GetProviderID() != s.Configuration.ProviderID {
		glog.Errorf(errMismatchingProvider)
		return nil, fmt.Errorf(errMismatchingProvider)
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
	glog.V(5).Infof("Call server Belongs: %v", request)

	if request.GetProviderID() != s.Configuration.ProviderID {
		glog.Errorf(errMismatchingProvider)
		return nil, fmt.Errorf(errMismatchingProvider)
	}

	node, err := nodeFromJSON(request.GetNode())

	if err != nil {
		glog.Errorf(errCantUnmarshallNodeWithReason, request.GetNode(), err)

		return &apigrpc.BelongsReply{
			Response: &apigrpc.BelongsReply_Error{
				Error: &apigrpc.Error{
					Code:   cloudProviderError,
					Reason: err.Error(),
				},
			},
		}, nil
	}

	providerID := getNodeProviderID(s.Configuration.ProviderID, node)
	nodeGroup, err := s.nodeGroupForNode(providerID)

	var belong bool

	if nodeGroup != nil {
		if nodeGroup.NodeGroupIdentifier == request.GetNodeGroupID() {
			nodeName, err := nodeNameFromProviderID(s.Configuration.ProviderID, providerID)

			if err != nil {
				return &apigrpc.BelongsReply{
					Response: &apigrpc.BelongsReply_Error{
						Error: &apigrpc.Error{
							Code:   cloudProviderError,
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
	glog.V(5).Infof("Call server NodePrice: %v", request)

	if request.GetProviderID() != s.Configuration.ProviderID {
		glog.Errorf(errMismatchingProvider)
		return nil, fmt.Errorf(errMismatchingProvider)
	}

	return &apigrpc.NodePriceReply{
		Response: &apigrpc.NodePriceReply_Price{
			Price: s.Configuration.NodePrice,
		},
	}, nil
}

// PodPrice returns a theoretical minimum price of running a pod for a given
// period of time on a perfectly matching machine.
func (s *AutoScalerServerApp) PodPrice(ctx context.Context, request *apigrpc.PodPriceRequest) (*apigrpc.PodPriceReply, error) {
	glog.V(5).Infof("Call server PodPrice: %v", request)

	if request.GetProviderID() != s.Configuration.ProviderID {
		glog.Errorf(errMismatchingProvider)
		return nil, fmt.Errorf(errMismatchingProvider)
	}

	return &apigrpc.PodPriceReply{
		Response: &apigrpc.PodPriceReply_Price{
			Price: s.Configuration.PodPrice,
		},
	}, nil
}

func (s *AutoScalerServerApp) save(fileName string) error {
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

func (s *AutoScalerServerApp) load(fileName string) error {
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

	if s.AutoProvision {
		if err := s.doAutoProvision(); err != nil {
			glog.Errorf(errUnableToAutoProvisionNodeGroup, err)

			return err
		}
	}

	return nil
}
