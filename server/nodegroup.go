package server

import (
	"encoding/json"
	"fmt"
	"strconv"
	"sync"

	"github.com/Fred78290/kubernetes-vmware-autoscaler/constantes"
	"github.com/Fred78290/kubernetes-vmware-autoscaler/types"
	"github.com/Fred78290/kubernetes-vmware-autoscaler/utils"
	"github.com/golang/glog"
	apiv1 "k8s.io/api/core/v1"
)

// NodeGroupState describe the nodegroup status
type NodeGroupState int32

const (
	// NodegroupNotCreated not created state
	NodegroupNotCreated NodeGroupState = 0

	// NodegroupCreated create state
	NodegroupCreated NodeGroupState = 1

	// NodegroupDeleting deleting status
	NodegroupDeleting NodeGroupState = 2

	// NodegroupDeleted deleted status
	NodegroupDeleted NodeGroupState = 3
)

// AutoScalerServerNodeGroup Group all AutoScaler VM created inside a NodeGroup
// Each node have name like <node group name>-vm-<vm index>
type AutoScalerServerNodeGroup struct {
	sync.Mutex
	NodeGroupIdentifier  string                           `json:"identifier"`
	ServiceIdentifier    string                           `json:"service"`
	Machine              *types.MachineCharacteristic     `json:"machine"`
	Status               NodeGroupState                   `json:"status"`
	MinNodeSize          int                              `json:"minSize"`
	MaxNodeSize          int                              `json:"maxSize"`
	Nodes                map[string]*AutoScalerServerNode `json:"nodes"`
	NodeLabels           map[string]string                `json:"nodeLabels"`
	SystemLabels         map[string]string                `json:"systemLabels"`
	AutoProvision        bool                             `json:"auto-provision"`
	LastCreatedNodeIndex int                              `json:"node-index"`
	PendingNodes         map[string]*AutoScalerServerNode `json:"-"`
	PendingNodesWG       sync.WaitGroup                   `json:"-"`
}

type nodeCreationExtra struct {
	nodegroupID   string
	kubeHost      string
	kubeToken     string
	kubeCACert    string
	kubeExtraArgs []string
	kubeConfig    string
	image         string
	cloudInit     map[string]interface{}
	syncFolders   *types.AutoScalerServerSyncFolders
	nodeLabels    map[string]string
	systemLabels  map[string]string
	vmprovision   bool
}

func (g *AutoScalerServerNodeGroup) cleanup(kubeconfig string) error {
	glog.V(5).Infof("AutoScalerServerNodeGroup::cleanup, nodeGroupID:%s", g.NodeGroupIdentifier)

	var lastError error

	g.Status = NodegroupDeleting

	g.PendingNodesWG.Wait()

	glog.V(5).Infof("AutoScalerServerNodeGroup::cleanup, nodeGroupID:%s, iterate node to delete", g.NodeGroupIdentifier)

	for _, node := range g.Nodes {
		if lastError = node.deleteVM(kubeconfig); lastError != nil {
			glog.Errorf(constantes.ErrNodeGroupCleanupFailOnVM, g.NodeGroupIdentifier, node.NodeName, lastError)
		}
	}

	g.Nodes = make(map[string]*AutoScalerServerNode)
	g.PendingNodes = make(map[string]*AutoScalerServerNode)
	g.Status = NodegroupDeleted

	return lastError
}

func (g *AutoScalerServerNodeGroup) targetSize() int {
	glog.V(5).Infof("AutoScalerServerNodeGroup::targetSize, nodeGroupID:%s", g.NodeGroupIdentifier)

	return len(g.PendingNodes) + len(g.Nodes)
}

func (g *AutoScalerServerNodeGroup) setNodeGroupSize(newSize int, extras *nodeCreationExtra) error {
	glog.V(5).Infof("AutoScalerServerNodeGroup::setNodeGroupSize, nodeGroupID:%s", g.NodeGroupIdentifier)

	var err error

	g.Lock()

	delta := newSize - g.targetSize()

	if delta < 0 {
		err = g.deleteNodes(delta, extras)
	} else if delta > 0 {
		err = g.addNodes(delta, extras)
	}

	g.Unlock()

	return err
}

func (g *AutoScalerServerNodeGroup) refresh() {
	glog.V(5).Infof("AutoScalerServerNodeGroup::refresh, nodeGroupID:%s", g.NodeGroupIdentifier)

	for _, node := range g.Nodes {
		node.statusVM()
	}
}

// delta must be negative!!!!
func (g *AutoScalerServerNodeGroup) deleteNodes(delta int, extras *nodeCreationExtra) error {
	glog.V(5).Infof("AutoScalerServerNodeGroup::deleteNodes, nodeGroupID:%s", g.NodeGroupIdentifier)

	startIndex := len(g.Nodes) - 1
	endIndex := startIndex + delta
	tempNodes := make([]*AutoScalerServerNode, 0, -delta)

	for nodeIndex := startIndex; nodeIndex >= endIndex; nodeIndex-- {
		nodeName := g.nodeName(nodeIndex)

		if node := g.Nodes[nodeName]; node != nil {
			if err := node.deleteVM(extras.kubeConfig); err != nil {
				glog.Errorf(constantes.ErrUnableToDeleteVM, node.NodeName, err)
				return err
			}

			tempNodes = append(tempNodes, node)
		}
	}

	for _, node := range tempNodes {
		delete(g.Nodes, node.NodeName)
	}

	return nil
}

func (g *AutoScalerServerNodeGroup) addNodes(delta int, extras *nodeCreationExtra) error {
	glog.V(5).Infof("AutoScalerServerNodeGroup::addNodes, nodeGroupID:%s", g.NodeGroupIdentifier)

	tempNodes := make([]*AutoScalerServerNode, 0, delta)

	g.PendingNodesWG.Add(delta)

	for nodeIndex := 0; nodeIndex < delta; nodeIndex++ {
		if g.Status != NodegroupCreated {
			glog.V(5).Infof("AutoScalerServerNodeGroup::addNodes, nodeGroupID:%s -> g.status != nodegroupCreated", g.NodeGroupIdentifier)
			break
		}

		g.LastCreatedNodeIndex++

		nodeName := g.nodeName(g.LastCreatedNodeIndex)

		node := &AutoScalerServerNode{
			ProviderID:       g.providerIDForNode(nodeName),
			NodeName:         nodeName,
			NodeIndex:        g.LastCreatedNodeIndex,
			Memory:           g.Machine.Memory,
			CPU:              g.Machine.Vcpu,
			Disk:             g.Machine.Disk,
			AutoProvisionned: true,
		}

		tempNodes = append(tempNodes, node)

		if g.PendingNodes == nil {
			g.PendingNodes = make(map[string]*AutoScalerServerNode)
		}

		g.PendingNodes[node.NodeName] = node
	}

	for _, node := range tempNodes {
		if g.Status != NodegroupCreated {
			glog.V(5).Infof("AutoScalerServerNodeGroup::addNodes, nodeGroupID:%s -> g.status != nodegroupCreated", g.NodeGroupIdentifier)
			break
		}

		if err := node.launchVM(extras); err != nil {
			glog.Errorf(constantes.ErrUnableToLaunchVM, node.NodeName, err)

			for _, node := range tempNodes {
				delete(g.PendingNodes, node.NodeName)

				if status, _ := node.statusVM(); status == AutoScalerServerNodeStateRunning {
					if err := node.deleteVM(extras.kubeConfig); err != nil {
						glog.Errorf(constantes.ErrUnableToDeleteVM, node.NodeName, err)
					}
				}

				g.PendingNodesWG.Done()
			}

			return err
		}

		delete(g.PendingNodes, node.NodeName)

		g.Nodes[node.NodeName] = node
		g.PendingNodesWG.Done()
	}

	return nil
}

func (g *AutoScalerServerNodeGroup) autoDiscoveryNodes(scaleDownDisabled bool, kubeconfig string) error {
	var lastNodeIndex = 0
	var nodeInfos apiv1.NodeList
	var out string
	var err error
	var arg = []string{
		"kubectl",
		"get",
		"nodes",
		"--output",
		"json",
		"--kubeconfig",
		kubeconfig,
	}

	if out, err = utils.Pipe(arg...); err != nil {
		return err
	}

	if err = json.Unmarshal([]byte(out), &nodeInfos); err != nil {
		return fmt.Errorf(constantes.ErrUnmarshallingError, "AutoScalerServerNodeGroup::autoDiscoveryNodes", err)
	}

	formerNodes := g.Nodes

	g.Nodes = make(map[string]*AutoScalerServerNode)
	g.PendingNodes = make(map[string]*AutoScalerServerNode)

	for _, nodeInfo := range nodeInfos.Items {
		var providerID = utils.GetNodeProviderID(g.ServiceIdentifier, &nodeInfo)
		var nodeID = ""

		if len(providerID) > 0 {
			out, err = utils.NodeGroupIDFromProviderID(g.ServiceIdentifier, providerID)

			if out == g.NodeGroupIdentifier {
				glog.Infof("Discover node:%s matching nodegroup:%s", providerID, g.NodeGroupIdentifier)

				if nodeID, err = utils.NodeNameFromProviderID(g.ServiceIdentifier, providerID); err == nil {
					node := formerNodes[nodeID]

					runningIP := ""

					for _, address := range nodeInfo.Status.Addresses {
						if address.Type == apiv1.NodeInternalIP {
							runningIP = address.Address
							break
						}
					}

					glog.Infof("Add node:%s with IP:%s to nodegroup:%s", nodeID, runningIP, g.NodeGroupIdentifier)

					if len(nodeInfo.Annotations[constantes.AnnotationNodeIndex]) != 0 {
						lastNodeIndex, _ = strconv.Atoi(nodeInfo.Annotations[constantes.AnnotationNodeIndex])
					}

					g.LastCreatedNodeIndex = utils.MaxInt(g.LastCreatedNodeIndex, lastNodeIndex)

					if node == nil {
						node = &AutoScalerServerNode{
							ProviderID:       providerID,
							NodeName:         nodeID,
							NodeIndex:        lastNodeIndex,
							State:            AutoScalerServerNodeStateRunning,
							AutoProvisionned: nodeInfo.Annotations[constantes.AnnotationNodeAutoProvisionned] == "true",
							Addresses: []string{
								runningIP,
							},
						}

						arg = []string{
							"kubectl",
							"annotate",
							"node",
							nodeInfo.Name,
							fmt.Sprintf("%s=%s", constantes.AnnotationScaleDownDisabled, strconv.FormatBool(scaleDownDisabled && node.AutoProvisionned == false)),
							fmt.Sprintf("%s=%s", constantes.AnnotationNodeAutoProvisionned, strconv.FormatBool(node.AutoProvisionned)),
							fmt.Sprintf("%s=%d", constantes.AnnotationNodeIndex, node.NodeIndex),
							"--overwrite",
							"--kubeconfig",
							kubeconfig,
						}

						if err := utils.Shell(arg...); err != nil {
							glog.Errorf(constantes.ErrKubeCtlIgnoredError, nodeInfo.Name, err)
						}

						arg = []string{
							"kubectl",
							"label",
							"nodes",
							nodeInfo.Name,
							fmt.Sprintf("%s=%s", constantes.NodeLabelGroupName, g.NodeGroupIdentifier),
							"--overwrite",
							"--kubeconfig",
							kubeconfig,
						}

						if err := utils.Shell(arg...); err != nil {
							glog.Errorf(constantes.ErrKubeCtlIgnoredError, nodeInfo.Name, err)
						}
					}

					lastNodeIndex++

					g.Nodes[nodeID] = node

					node.statusVM()
				}
			}
		}
	}

	return nil
}

func (g *AutoScalerServerNodeGroup) deleteNodeByName(kubeconfig, nodeName string) error {
	glog.V(5).Infof("AutoScalerServerNodeGroup::deleteNodeByName, nodeGroupID:%s, nodeName:%s", g.NodeGroupIdentifier, nodeName)

	if node := g.Nodes[nodeName]; node != nil {

		if err := node.deleteVM(kubeconfig); err != nil {
			glog.Errorf(constantes.ErrUnableToDeleteVM, node.NodeName, err)
			return err
		}

		delete(g.Nodes, nodeName)

		return nil
	}

	return fmt.Errorf(constantes.ErrNodeNotFoundInNodeGroup, nodeName, g.NodeGroupIdentifier)
}

func (g *AutoScalerServerNodeGroup) deleteNodeGroup(kubeConfig string) error {
	glog.V(5).Infof("AutoScalerServerNodeGroup::deleteNodeGroup, nodeGroupID:%s", g.NodeGroupIdentifier)

	return g.cleanup(kubeConfig)
}

func (g *AutoScalerServerNodeGroup) nodeName(vmIndex int) string {
	return fmt.Sprintf("%s-vm-%02d", g.NodeGroupIdentifier, vmIndex)
}

func (g *AutoScalerServerNodeGroup) providerID() string {
	return fmt.Sprintf("%s://%s/object?type=group", g.ServiceIdentifier, g.NodeGroupIdentifier)
}

func (g *AutoScalerServerNodeGroup) providerIDForNode(nodeName string) string {
	return fmt.Sprintf("%s://%s/object?type=node&name=%s", g.ServiceIdentifier, g.NodeGroupIdentifier, nodeName)
}
