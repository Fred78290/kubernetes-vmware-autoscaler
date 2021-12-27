package server

import (
	"fmt"
	"strconv"
	"sync"

	"github.com/Fred78290/kubernetes-vmware-autoscaler/api/types/v1alpha1"
	"github.com/Fred78290/kubernetes-vmware-autoscaler/constantes"
	"github.com/Fred78290/kubernetes-vmware-autoscaler/types"
	"github.com/Fred78290/kubernetes-vmware-autoscaler/utils"
	glog "github.com/sirupsen/logrus"
	apiv1 "k8s.io/api/core/v1"
	uid "k8s.io/apimachinery/pkg/types"
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

// KubernetesLabel labels
type KubernetesLabel map[string]string

type ServerNodeState int

const (
	ServerNodeStateNotRunning ServerNodeState = 0
	ServerNodeStateDeleted    ServerNodeState = 1
	ServerNodeStateCreating   ServerNodeState = 2
	ServerNodeStateRunning    ServerNodeState = 3
)

// AutoScalerServerNodeGroup Group all AutoScaler VM created inside a NodeGroup
// Each node have name like <node group name>-vm-<vm index>
type AutoScalerServerNodeGroup struct {
	sync.Mutex
	NodeGroupIdentifier        string                           `json:"identifier"`
	ServiceIdentifier          string                           `json:"service"`
	ProvisionnedNodeNamePrefix string                           `default:"autoscaled" json:"node-name-prefix"`
	ManagedNodeNamePrefix      string                           `default:"autoscaled" json:"managed-name-prefix"`
	Machine                    *types.MachineCharacteristic     `json:"machine"`
	Status                     NodeGroupState                   `json:"status"`
	MinNodeSize                int                              `json:"minSize"`
	MaxNodeSize                int                              `json:"maxSize"`
	Nodes                      map[string]*AutoScalerServerNode `json:"nodes"`
	NodeLabels                 KubernetesLabel                  `json:"nodeLabels"`
	SystemLabels               KubernetesLabel                  `json:"systemLabels"`
	AutoProvision              bool                             `json:"auto-provision"`
	LastCreatedNodeIndex       int                              `json:"node-index"`
	RunningNode                map[int]ServerNodeState          `json:"running-nodes-state"`
	pendingNodes               map[string]*AutoScalerServerNode
	pendingNodesWG             sync.WaitGroup
	numOfExternalNodes         int
	numOfProvisionnedNodes     int
	numOfManagedNodes          int
	configuration              *types.AutoScalerServerConfig
}

func (g *AutoScalerServerNodeGroup) findNextNodeIndex(managed bool) int {

	for index := 1; index <= g.MaxNodeSize; index++ {
		if run, found := g.RunningNode[index]; !found || run < ServerNodeStateCreating {
			return index
		}
	}

	g.LastCreatedNodeIndex++

	return g.LastCreatedNodeIndex
}

func (g *AutoScalerServerNodeGroup) cleanup(c types.ClientGenerator) error {
	glog.Debugf("AutoScalerServerNodeGroup::cleanup, nodeGroupID:%s", g.NodeGroupIdentifier)

	var lastError error

	g.Status = NodegroupDeleting

	g.pendingNodesWG.Wait()

	glog.Debugf("AutoScalerServerNodeGroup::cleanup, nodeGroupID:%s, iterate node to delete", g.NodeGroupIdentifier)

	for _, node := range g.Nodes {
		if node.AutoProvisionned {
			if lastError = node.deleteVM(c); lastError != nil {
				glog.Errorf(constantes.ErrNodeGroupCleanupFailOnVM, g.NodeGroupIdentifier, node.NodeName, lastError)
			}
		}
	}

	g.RunningNode = make(map[int]ServerNodeState)
	g.Nodes = make(map[string]*AutoScalerServerNode)
	g.pendingNodes = make(map[string]*AutoScalerServerNode)
	g.numOfExternalNodes = 0
	g.numOfManagedNodes = 0
	g.numOfProvisionnedNodes = 0
	g.Status = NodegroupDeleted

	return lastError
}

func (g *AutoScalerServerNodeGroup) targetSize() int {
	glog.Debugf("AutoScalerServerNodeGroup::targetSize, nodeGroupID:%s", g.NodeGroupIdentifier)

	return len(g.pendingNodes) + len(g.Nodes)
}

func (g *AutoScalerServerNodeGroup) setNodeGroupSize(c types.ClientGenerator, newSize int) error {
	glog.Debugf("AutoScalerServerNodeGroup::setNodeGroupSize, nodeGroupID:%s", g.NodeGroupIdentifier)

	var err error

	g.Lock()

	delta := newSize - g.targetSize()

	if delta < 0 {
		err = g.deleteNodes(c, delta)
	} else if delta > 0 {
		err = g.addNodes(c, delta)
	}

	g.Unlock()

	return err
}

func (g *AutoScalerServerNodeGroup) refresh() {
	glog.Debugf("AutoScalerServerNodeGroup::refresh, nodeGroupID:%s", g.NodeGroupIdentifier)

	for _, node := range g.Nodes {
		if _, err := node.statusVM(); err != nil {
			glog.Infof("status VM return an error: %v", err)
		}
	}
}

// delta must be negative!!!!
func (g *AutoScalerServerNodeGroup) deleteNodes(c types.ClientGenerator, delta int) error {
	glog.Debugf("AutoScalerServerNodeGroup::deleteNodes, nodeGroupID:%s", g.NodeGroupIdentifier)

	var err error

	startIndex := len(g.Nodes) - 1
	endIndex := startIndex + delta
	tempNodes := make([]*AutoScalerServerNode, 0, -delta)

	for index := startIndex; index >= endIndex; index-- {
		nodeName := g.nodeName(index, false)

		if node := g.Nodes[nodeName]; node != nil {
			tempNodes = append(tempNodes, node)

			if err = node.deleteVM(c); err != nil {
				glog.Errorf(constantes.ErrUnableToDeleteVM, node.NodeName, err)
				break
			}
		}
	}

	for _, node := range tempNodes {
		g.RunningNode[node.NodeIndex] = ServerNodeStateDeleted
		delete(g.Nodes, node.NodeName)
	}

	return err
}

func (g *AutoScalerServerNodeGroup) addManagedNode(crd *v1alpha1.ManagedNode) (*AutoScalerServerNode, error) {
	nodeIndex := g.findNextNodeIndex(true)
	nodeName := g.nodeName(nodeIndex, true)

	// Clone the vsphere config to allow increment IP address
	if vsphereConfig, err := g.configuration.GetVSphereConfiguration(g.NodeGroupIdentifier).Clone(nodeIndex); err == nil {
		node := &AutoScalerServerNode{
			ProviderID:       g.providerIDForNode(nodeName),
			NodeGroupID:      g.NodeGroupIdentifier,
			NodeName:         nodeName,
			NodeIndex:        nodeIndex,
			Memory:           utils.MinInt(2048, crd.Spec.MemorySize),
			CPU:              utils.MaxInt(32, utils.MinInt(1, crd.Spec.VCpus)),
			Disk:             utils.MinInt(crd.Spec.DiskSize, 10240),
			AutoProvisionned: false,
			UID:              crd.GetUID(),
			ManagedNode:      true,
			VSphereConfig:    vsphereConfig,
			serverConfig:     g.configuration,
		}
		return node, nil
	} else {
		return nil, err
	}
}

func (g *AutoScalerServerNodeGroup) addNodes(c types.ClientGenerator, delta int) error {
	tempNodes := make([]*AutoScalerServerNode, 0, delta)

	if g.Status != NodegroupCreated {
		glog.Debugf("AutoScalerServerNodeGroup::addNodes, nodeGroupID:%s -> g.status != nodegroupCreated", g.NodeGroupIdentifier)
		return fmt.Errorf(constantes.ErrNodeGroupNotFound, g.NodeGroupIdentifier)
	}

	for index := 0; index < delta; index++ {
		nodeIndex := g.findNextNodeIndex(false)
		nodeName := g.nodeName(nodeIndex, false)

		// Clone the vsphere config to allow increment IP address
		if vsphereConfig, err := g.configuration.GetVSphereConfiguration(g.NodeGroupIdentifier).Clone(nodeIndex); err == nil {

			g.RunningNode[nodeIndex] = ServerNodeStateCreating

			node := &AutoScalerServerNode{
				ProviderID:       g.providerIDForNode(nodeName),
				NodeGroupID:      g.NodeGroupIdentifier,
				NodeName:         nodeName,
				NodeIndex:        nodeIndex,
				Memory:           g.Machine.Memory,
				CPU:              g.Machine.Vcpu,
				Disk:             g.Machine.Disk,
				AutoProvisionned: true,
				ManagedNode:      false,
				VSphereConfig:    vsphereConfig,
				serverConfig:     g.configuration,
			}

			tempNodes = append(tempNodes, node)

			if g.pendingNodes == nil {
				g.pendingNodes = make(map[string]*AutoScalerServerNode)
			}

			g.pendingNodes[node.NodeName] = node
		} else {
			g.pendingNodes = nil
			return err
		}
	}

	return g.createNodes(c, tempNodes)
}

func (g *AutoScalerServerNodeGroup) createNodes(c types.ClientGenerator, nodes []*AutoScalerServerNode) error {
	var mu sync.Mutex

	glog.Debugf("AutoScalerServerNodeGroup::addNodes, nodeGroupID:%s", g.NodeGroupIdentifier)

	numberOfNodesToCreate := len(nodes)

	createNode := func(node *AutoScalerServerNode) error {
		var err error

		defer g.pendingNodesWG.Done()

		if g.Status != NodegroupCreated {
			glog.Debugf("AutoScalerServerNodeGroup::addNodes, nodeGroupID:%s -> g.status != nodegroupCreated", g.NodeGroupIdentifier)
			return fmt.Errorf(constantes.ErrUnableToLaunchVMNodeGroupNotReady, node.NodeName)
		}

		if err = node.launchVM(c, g.NodeLabels, g.SystemLabels); err != nil {
			glog.Errorf(constantes.ErrUnableToLaunchVM, node.NodeName, err)

			if status, _ := node.statusVM(); status != AutoScalerServerNodeStateNotCreated {
				if e := node.deleteVM(c); e != nil {
					glog.Errorf(constantes.ErrUnableToDeleteVM, node.NodeName, e)
				}
			} else {
				glog.Warningf(constantes.WarnFailedVMNotDeleted, node.NodeName, status)
			}

			mu.Lock()
			defer mu.Unlock()

			g.RunningNode[node.NodeIndex] = ServerNodeStateDeleted
		} else {
			mu.Lock()
			defer mu.Unlock()

			g.Nodes[node.NodeName] = node
			g.RunningNode[node.NodeIndex] = ServerNodeStateRunning

			if node.AutoProvisionned {
				g.numOfProvisionnedNodes++
			} else if node.ManagedNode {
				g.numOfManagedNodes++
			}
		}

		delete(g.pendingNodes, node.NodeName)

		return err
	}

	var result error = nil
	var successful int

	g.pendingNodesWG.Add(numberOfNodesToCreate)

	// Do sync if one node only
	if numberOfNodesToCreate == 1 {
		if err := createNode(nodes[0]); err == nil {
			successful++
		}
	} else {
		var maxCreatedNodePerCycle int

		if g.configuration.MaxCreatedNodePerCycle <= 0 {
			maxCreatedNodePerCycle = numberOfNodesToCreate
		} else {
			maxCreatedNodePerCycle = g.configuration.MaxCreatedNodePerCycle
		}

		glog.Debugf("Launch node group %s of %d VM by %d nodes per cycle", g.NodeGroupIdentifier, numberOfNodesToCreate, maxCreatedNodePerCycle)

		createNodeAsync := func(currentNode *AutoScalerServerNode, wg *sync.WaitGroup, err chan error) {
			e := createNode(currentNode)
			wg.Done()
			err <- e
		}

		totalLoop := numberOfNodesToCreate / maxCreatedNodePerCycle

		if numberOfNodesToCreate%maxCreatedNodePerCycle > 0 {
			totalLoop++
		}

		currentNodeIndex := 0

		for numberOfCycle := 0; numberOfCycle < totalLoop; numberOfCycle++ {
			// WaitGroup per segment
			numberOfNodeInCycle := utils.MinInt(maxCreatedNodePerCycle, numberOfNodesToCreate-currentNodeIndex)

			glog.Debugf("Launched cycle: %d, node per cycle is: %d", numberOfCycle, numberOfNodeInCycle)

			if numberOfNodeInCycle > 1 {
				ch := make([]chan error, numberOfNodeInCycle)
				wg := sync.WaitGroup{}

				for nodeInCycle := 0; nodeInCycle < numberOfNodeInCycle; nodeInCycle++ {
					node := nodes[currentNodeIndex]
					cherr := make(chan error)
					ch[nodeInCycle] = cherr

					currentNodeIndex++

					wg.Add(1)

					go createNodeAsync(node, &wg, cherr)
				}

				// Wait this segment to launch
				glog.Debugf("Wait cycle to finish: %d", numberOfCycle)

				wg.Wait()

				glog.Debugf("Launched cycle: %d, collect result", numberOfCycle)

				for _, chError := range ch {
					if chError != nil {
						var err = <-chError

						if err == nil {
							successful++
						}
					}
				}

				glog.Debugf("Finished cycle: %d", numberOfCycle)
			} else if numberOfNodeInCycle > 0 {
				node := nodes[currentNodeIndex]
				currentNodeIndex++
				if err := createNode(node); err == nil {
					successful++
				}
			} else {
				break
			}
		}
	}

	g.pendingNodesWG.Wait()

	if g.Status != NodegroupCreated {
		result = fmt.Errorf(constantes.ErrUnableToLaunchNodeGroupNotCreated, g.NodeGroupIdentifier)

		glog.Errorf("Launched node group %s of %d VM got an error because was destroyed", g.NodeGroupIdentifier, numberOfNodesToCreate)
	} else if successful == 0 {
		result = fmt.Errorf(constantes.ErrUnableToLaunchNodeGroup, g.NodeGroupIdentifier)
		glog.Infof("Launched node group %s of %d VM failed", g.NodeGroupIdentifier, numberOfNodesToCreate)
	} else {
		glog.Infof("Launched node group %s of %d/%d VM successful", g.NodeGroupIdentifier, successful, numberOfNodesToCreate)
	}

	return result
}

func (g *AutoScalerServerNodeGroup) autoDiscoveryNodes(client types.ClientGenerator, includeExistingNode bool) error {
	var lastNodeIndex = 0
	var nodeInfos *apiv1.NodeList
	var out string
	var err error

	if nodeInfos, err = client.NodeList(); err != nil {
		return err
	}

	formerNodes := g.Nodes

	g.Nodes = make(map[string]*AutoScalerServerNode)
	g.pendingNodes = make(map[string]*AutoScalerServerNode)
	g.RunningNode = make(map[int]ServerNodeState)
	g.LastCreatedNodeIndex = 0
	g.numOfExternalNodes = 0
	g.numOfManagedNodes = 0
	g.numOfProvisionnedNodes = 0

	for _, nodeInfo := range nodeInfos.Items {
		var providerID = utils.GetNodeProviderID(g.ServiceIdentifier, &nodeInfo)
		var nodeID string

		if len(providerID) > 0 {
			out, _ = utils.NodeGroupIDFromProviderID(g.ServiceIdentifier, providerID)

			autoProvisionned, _ := strconv.ParseBool(nodeInfo.Annotations[constantes.AnnotationNodeAutoProvisionned])
			managedNode, _ := strconv.ParseBool(nodeInfo.Annotations[constantes.AnnotationNodeManaged])

			// Ignore nodes not handled by autoscaler if option includeExistingNode == false
			if out == g.NodeGroupIdentifier && (autoProvisionned || includeExistingNode) {
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
							NodeGroupID:      g.NodeGroupIdentifier,
							NodeName:         nodeID,
							NodeIndex:        lastNodeIndex,
							State:            AutoScalerServerNodeStateRunning,
							AutoProvisionned: autoProvisionned,
							ManagedNode:      managedNode,
							VSphereConfig:    g.configuration.GetVSphereConfiguration(g.NodeGroupIdentifier).Copy(),
							Addresses: []string{
								runningIP,
							},
							serverConfig: g.configuration,
						}

						err = client.AnnoteNode(nodeInfo.Name, map[string]string{
							constantes.AnnotationScaleDownDisabled:    strconv.FormatBool(!autoProvisionned),
							constantes.AnnotationNodeAutoProvisionned: strconv.FormatBool(autoProvisionned),
							constantes.AnnotationNodeManaged:          strconv.FormatBool(node.ManagedNode),
							constantes.AnnotationNodeIndex:            strconv.Itoa(node.NodeIndex),
						})

						if err != nil {
							glog.Errorf(constantes.ErrAnnoteNodeReturnError, nodeInfo.Name, err)
						}

						err = client.LabelNode(nodeInfo.Name, map[string]string{
							constantes.NodeLabelGroupName: g.NodeGroupIdentifier,
						})

						if err != nil {
							glog.Errorf(constantes.ErrLabelNodeReturnError, nodeInfo.Name, err)
						}
					}

					node.retrieveNetworkInfos()

					g.Nodes[nodeID] = node
					g.RunningNode[lastNodeIndex] = ServerNodeStateRunning

					if autoProvisionned {
						g.numOfProvisionnedNodes++
					} else if managedNode {
						g.numOfManagedNodes++
					} else {
						g.numOfExternalNodes++
					}

					lastNodeIndex++

					_, _ = node.statusVM()
				} else {
					glog.Errorf(constantes.ErrGetVMInfoFailed, nodeInfo.Name, err)
				}
			} else {
				glog.Infof("Ignore kubernetes node %s not handled by me", nodeInfo.Name)
			}
		}
	}

	return nil
}

func (g *AutoScalerServerNodeGroup) deleteNode(c types.ClientGenerator, node *AutoScalerServerNode) error {
	var err error

	if err = node.deleteVM(c); err != nil {
		glog.Errorf(constantes.ErrUnableToDeleteVM, node.NodeName, err)
	}

	g.RunningNode[node.NodeIndex] = ServerNodeStateDeleted
	delete(g.Nodes, node.NodeName)

	if node.AutoProvisionned {
		g.numOfProvisionnedNodes--
	} else {
		g.numOfManagedNodes--
	}

	return err
}

func (g *AutoScalerServerNodeGroup) deleteNodeByName(c types.ClientGenerator, nodeName string) error {
	glog.Debugf("AutoScalerServerNodeGroup::deleteNodeByName, nodeGroupID:%s, nodeName:%s", g.NodeGroupIdentifier, nodeName)

	if node := g.Nodes[nodeName]; node != nil {

		return g.deleteNode(c, node)
	}

	return fmt.Errorf(constantes.ErrNodeNotFoundInNodeGroup, nodeName, g.NodeGroupIdentifier)
}

func (g *AutoScalerServerNodeGroup) setConfiguration(config *types.AutoScalerServerConfig) {
	glog.Debugf("AutoScalerServerNodeGroup::setConfiguration, nodeGroupID:%s", g.NodeGroupIdentifier)

	g.configuration = config

	for _, node := range g.Nodes {
		node.setServerConfiguration(config)
	}
}

func (g *AutoScalerServerNodeGroup) deleteNodeGroup(c types.ClientGenerator) error {
	glog.Debugf("AutoScalerServerNodeGroup::deleteNodeGroup, nodeGroupID:%s", g.NodeGroupIdentifier)

	return g.cleanup(c)
}

func (g *AutoScalerServerNodeGroup) getProvisionnedNodePrefix() string {
	if len(g.ProvisionnedNodeNamePrefix) == 0 {
		return "autoscaled"
	}

	return g.ProvisionnedNodeNamePrefix
}

func (g *AutoScalerServerNodeGroup) getManagedNodePrefix() string {
	if len(g.ManagedNodeNamePrefix) == 0 {
		return "worker"
	}

	return g.ManagedNodeNamePrefix
}

func (g *AutoScalerServerNodeGroup) nodeName(vmIndex int, managed bool) string {
	if managed {
		return fmt.Sprintf("%s-%s-%02d", g.NodeGroupIdentifier, g.getManagedNodePrefix(), vmIndex-g.numOfExternalNodes-g.numOfProvisionnedNodes)
	} else {
		return fmt.Sprintf("%s-%s-%02d", g.NodeGroupIdentifier, g.getProvisionnedNodePrefix(), vmIndex-g.numOfExternalNodes-g.numOfManagedNodes)
	}
}

func (g *AutoScalerServerNodeGroup) providerID() string {
	return fmt.Sprintf("%s://%s/object?type=group", g.ServiceIdentifier, g.NodeGroupIdentifier)
}

func (g *AutoScalerServerNodeGroup) providerIDForNode(nodeName string) string {
	return fmt.Sprintf("%s://%s/object?type=node&name=%s", g.ServiceIdentifier, g.NodeGroupIdentifier, nodeName)
}

func (g *AutoScalerServerNodeGroup) findNodeByUID(uid uid.UID) (*AutoScalerServerNode, error) {
	for _, node := range g.Nodes {
		if node.UID == uid {
			return node, nil
		}
	}
	return nil, fmt.Errorf(constantes.ErrManagedNodeNotFound, uid)
}
