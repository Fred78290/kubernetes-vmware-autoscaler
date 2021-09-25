package server

import (
	"fmt"
	"strconv"
	"sync"

	"github.com/Fred78290/kubernetes-vmware-autoscaler/constantes"
	"github.com/Fred78290/kubernetes-vmware-autoscaler/types"
	"github.com/Fred78290/kubernetes-vmware-autoscaler/utils"
	glog "github.com/sirupsen/logrus"
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
	NodeGroupIdentifier  string                           `json:"identifier"`
	ServiceIdentifier    string                           `json:"service"`
	Machine              *types.MachineCharacteristic     `json:"machine"`
	Status               NodeGroupState                   `json:"status"`
	MinNodeSize          int                              `json:"minSize"`
	MaxNodeSize          int                              `json:"maxSize"`
	Nodes                map[string]*AutoScalerServerNode `json:"nodes"`
	NodeLabels           KubernetesLabel                  `json:"nodeLabels"`
	SystemLabels         KubernetesLabel                  `json:"systemLabels"`
	AutoProvision        bool                             `json:"auto-provision"`
	LastCreatedNodeIndex int                              `json:"node-index"`
	RunningNode          map[int]ServerNodeState          `json:"node-map"`
	pendingNodes         map[string]*AutoScalerServerNode
	pendingNodesWG       sync.WaitGroup
	configuration        *types.AutoScalerServerConfig
}

func (g *AutoScalerServerNodeGroup) findNextNodeIndex() int {
	g.LastCreatedNodeIndex++

	for index := 1; index <= g.MaxNodeSize; index++ {
		if run, found := g.RunningNode[index]; !found || run < ServerNodeStateCreating {
			return index
		}
	}

	return g.LastCreatedNodeIndex
}

func (g *AutoScalerServerNodeGroup) cleanup(c types.ClientGenerator) error {
	glog.Debugf("AutoScalerServerNodeGroup::cleanup, nodeGroupID:%s", g.NodeGroupIdentifier)

	var lastError error

	g.Status = NodegroupDeleting

	g.pendingNodesWG.Wait()

	glog.Debugf("AutoScalerServerNodeGroup::cleanup, nodeGroupID:%s, iterate node to delete", g.NodeGroupIdentifier)

	for _, node := range g.Nodes {
		if lastError = node.deleteVM(c); lastError != nil {
			glog.Errorf(constantes.ErrNodeGroupCleanupFailOnVM, g.NodeGroupIdentifier, node.NodeName, lastError)
		}
	}

	g.Nodes = make(map[string]*AutoScalerServerNode)
	g.pendingNodes = make(map[string]*AutoScalerServerNode)
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
		_, _ = node.statusVM()
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
		nodeName := g.nodeName(index)

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

func (g *AutoScalerServerNodeGroup) addNodes(c types.ClientGenerator, delta int) error {
	glog.Debugf("AutoScalerServerNodeGroup::addNodes, nodeGroupID:%s", g.NodeGroupIdentifier)

	tempNodes := make([]*AutoScalerServerNode, 0, delta)

	for index := 0; index < delta; index++ {
		if g.Status != NodegroupCreated {
			glog.Debugf("AutoScalerServerNodeGroup::addNodes, nodeGroupID:%s -> g.status != nodegroupCreated", g.NodeGroupIdentifier)
			break
		}

		nodeIndex := g.findNextNodeIndex()
		nodeName := g.nodeName(nodeIndex)

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
				VSphereConfig:    vsphereConfig,
				serverConfig:     g.configuration,
			}

			tempNodes = append(tempNodes, node)

			if g.pendingNodes == nil {
				g.pendingNodes = make(map[string]*AutoScalerServerNode)
			}

			g.pendingNodes[node.NodeName] = node
		} else {
			return err
		}
	}

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
			g.RunningNode[node.NodeIndex] = ServerNodeStateDeleted
		} else {
			g.Nodes[node.NodeName] = node
			g.RunningNode[node.NodeIndex] = ServerNodeStateRunning
		}

		delete(g.pendingNodes, node.NodeName)

		return err
	}

	var result error = nil
	var successful int

	g.pendingNodesWG.Add(delta)

	// Do sync if one node only
	if len(tempNodes) == 1 {
		if err := createNode(tempNodes[0]); err == nil {
			successful++
		}
	} else {
		var maxCreatedNodePerCycle int

		if g.configuration.MaxCreatedNodePerCycle <= 0 {
			maxCreatedNodePerCycle = len(tempNodes)
		} else {
			maxCreatedNodePerCycle = g.configuration.MaxCreatedNodePerCycle
		}

		glog.Debugf("Launch node group %s of %d VM by %d nodes per cycle", g.NodeGroupIdentifier, delta, maxCreatedNodePerCycle)

		createNodeAsync := func(currentNode *AutoScalerServerNode, wg *sync.WaitGroup, err chan error) {
			e := createNode(currentNode)
			wg.Done()
			err <- e
		}

		totalLoop := len(tempNodes) / maxCreatedNodePerCycle

		if len(tempNodes)%maxCreatedNodePerCycle > 0 {
			totalLoop++
		}

		currentNodeIndex := 0

		for numberOfCycle := 0; numberOfCycle < totalLoop; numberOfCycle++ {
			// WaitGroup per segment
			numberOfNodeInCycle := utils.MinInt(maxCreatedNodePerCycle, len(tempNodes)-currentNodeIndex)

			glog.Debugf("Launched cycle: %d, node per cycle is: %d", numberOfCycle, numberOfNodeInCycle)

			if numberOfNodeInCycle > 1 {
				ch := make([]chan error, numberOfNodeInCycle)
				wg := sync.WaitGroup{}

				for nodeInCycle := 0; nodeInCycle < numberOfNodeInCycle; nodeInCycle++ {
					node := tempNodes[currentNodeIndex]
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
				node := tempNodes[currentNodeIndex]
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

		glog.Errorf("Launched node group %s of %d VM got an error because was destroyed", g.NodeGroupIdentifier, delta)
	} else if successful == 0 {
		result = fmt.Errorf(constantes.ErrUnableToLaunchNodeGroup, g.NodeGroupIdentifier)
		glog.Infof("Launched node group %s of %d VM failed", g.NodeGroupIdentifier, delta)
	} else {
		glog.Infof("Launched node group %s of %d/%d VM successful", g.NodeGroupIdentifier, successful, delta)
	}

	return result
}

func (g *AutoScalerServerNodeGroup) autoDiscoveryNodes(client types.ClientGenerator, scaleDownDisabled bool) error {
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

	for _, nodeInfo := range nodeInfos.Items {
		var providerID = utils.GetNodeProviderID(g.ServiceIdentifier, &nodeInfo)
		var nodeID string

		if len(providerID) > 0 {
			out, _ = utils.NodeGroupIDFromProviderID(g.ServiceIdentifier, providerID)

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
							NodeGroupID:      g.NodeGroupIdentifier,
							NodeName:         nodeID,
							NodeIndex:        lastNodeIndex,
							State:            AutoScalerServerNodeStateRunning,
							AutoProvisionned: nodeInfo.Annotations[constantes.AnnotationNodeAutoProvisionned] == "true",
							VSphereConfig:    g.configuration.GetVSphereConfiguration(g.NodeGroupIdentifier).Copy(),
							Addresses: []string{
								runningIP,
							},
							serverConfig: g.configuration,
						}

						err = client.AnnoteNode(nodeInfo.Name, map[string]string{
							constantes.AnnotationScaleDownDisabled:    strconv.FormatBool(scaleDownDisabled && !node.AutoProvisionned),
							constantes.AnnotationNodeAutoProvisionned: strconv.FormatBool(node.AutoProvisionned),
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

					lastNodeIndex++

					_, _ = node.statusVM()
				}
			}
		}
	}

	return nil
}

func (g *AutoScalerServerNodeGroup) deleteNodeByName(c types.ClientGenerator, nodeName string) error {
	glog.Debugf("AutoScalerServerNodeGroup::deleteNodeByName, nodeGroupID:%s, nodeName:%s", g.NodeGroupIdentifier, nodeName)

	var err error

	if node := g.Nodes[nodeName]; node != nil {

		if err = node.deleteVM(c); err != nil {
			glog.Errorf(constantes.ErrUnableToDeleteVM, node.NodeName, err)
		}

		g.RunningNode[node.NodeIndex] = ServerNodeStateDeleted
		delete(g.Nodes, nodeName)

		return err
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

func (g *AutoScalerServerNodeGroup) nodeName(vmIndex int) string {
	return fmt.Sprintf("%s-vm-%02d", g.NodeGroupIdentifier, vmIndex)
}

func (g *AutoScalerServerNodeGroup) providerID() string {
	return fmt.Sprintf("%s://%s/object?type=group", g.ServiceIdentifier, g.NodeGroupIdentifier)
}

func (g *AutoScalerServerNodeGroup) providerIDForNode(nodeName string) string {
	return fmt.Sprintf("%s://%s/object?type=node&name=%s", g.ServiceIdentifier, g.NodeGroupIdentifier, nodeName)
}
