package server

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/Fred78290/kubernetes-vmware-autoscaler/constantes"
	managednodeClientset "github.com/Fred78290/kubernetes-vmware-autoscaler/pkg/generated/clientset/versioned"
	"github.com/Fred78290/kubernetes-vmware-autoscaler/types"
	"github.com/Fred78290/kubernetes-vmware-autoscaler/utils"
	"github.com/Fred78290/kubernetes-vmware-autoscaler/vsphere"
	"github.com/stretchr/testify/assert"
	apiv1 "k8s.io/api/core/v1"
	apiextension "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type baseTest struct {
	testConfig *vsphere.Configuration
	t          *testing.T
}

type nodegroupTest struct {
	baseTest
}

type autoScalerServerNodeGroupTest struct {
	AutoScalerServerNodeGroup
	baseTest
}

func (ng *autoScalerServerNodeGroupTest) createTestNode(nodeName string, desiredState ...AutoScalerServerNodeState) *AutoScalerServerNode {
	var state AutoScalerServerNodeState = AutoScalerServerNodeStateNotCreated

	if len(desiredState) > 0 {
		state = desiredState[0]
	}

	node := &AutoScalerServerNode{
		NodeGroupID:   testGroupID,
		NodeName:      nodeName,
		VMUUID:        testVMUUID,
		CRDUID:        testCRDUID,
		Memory:        ng.Machine.Memory,
		CPU:           ng.Machine.Vcpu,
		Disk:          ng.Machine.Disk,
		IPAddress:     "127.0.0.1",
		State:         state,
		NodeType:      AutoScalerServerNodeAutoscaled,
		NodeIndex:     1,
		VSphereConfig: ng.testConfig,
		serverConfig:  ng.configuration,
	}

	if vmuuid := node.findInstanceUUID(); len(vmuuid) > 0 {
		node.VMUUID = vmuuid
	}

	ng.Nodes[nodeName] = node
	ng.RunningNodes[len(ng.RunningNodes)+1] = ServerNodeStateRunning

	return node
}

func (m *nodegroupTest) launchVM() {
	ng, testNode, err := m.newTestNode(launchVMName)

	if assert.NoError(m.t, err) {
		if err := testNode.launchVM(m, ng.NodeLabels, ng.SystemLabels); err != nil {
			m.t.Errorf("AutoScalerNode.launchVM() error = %v", err)
		}
	}
}

func (m *nodegroupTest) startVM() {
	_, testNode, err := m.newTestNode(launchVMName)

	if assert.NoError(m.t, err) {
		if err := testNode.startVM(m); err != nil {
			m.t.Errorf("AutoScalerNode.startVM() error = %v", err)
		}
	}
}

func (m *nodegroupTest) stopVM() {
	_, testNode, err := m.newTestNode(launchVMName)

	if assert.NoError(m.t, err) {
		if err := testNode.stopVM(m); err != nil {
			m.t.Errorf("AutoScalerNode.stopVM() error = %v", err)
		}
	}
}

func (m *nodegroupTest) deleteVM() {
	_, testNode, err := m.newTestNode(launchVMName)

	if assert.NoError(m.t, err) {
		if err := testNode.deleteVM(m); err != nil {
			m.t.Errorf("AutoScalerNode.deleteVM() error = %v", err)
		}
	}
}

func (m *nodegroupTest) statusVM() {
	_, testNode, err := m.newTestNode(launchVMName)

	if assert.NoError(m.t, err) {
		if got, err := testNode.statusVM(); err != nil {
			m.t.Errorf("AutoScalerNode.statusVM() error = %v", err)
		} else if got != AutoScalerServerNodeStateRunning {
			m.t.Errorf("AutoScalerNode.statusVM() = %v, want %v", got, AutoScalerServerNodeStateRunning)
		}
	}
}

func (m *nodegroupTest) addNode() {
	ng, err := m.newTestNodeGroup()

	if assert.NoError(m.t, err) {
		if _, err := ng.addNodes(m, 1); err != nil {
			m.t.Errorf("AutoScalerServerNodeGroup.addNode() error = %v", err)
		}
	}
}

func (m *nodegroupTest) deleteNode() {
	ng, testNode, err := m.newTestNode(launchVMName)

	if assert.NoError(m.t, err) {
		if err := ng.deleteNodeByName(m, testNode.NodeName); err != nil {
			m.t.Errorf("AutoScalerServerNodeGroup.deleteNode() error = %v", err)
		}
	}
}

func (m *nodegroupTest) deleteNodeGroup() {
	ng, err := m.newTestNodeGroup()

	if assert.NoError(m.t, err) {
		if err := ng.deleteNodeGroup(m); err != nil {
			m.t.Errorf("AutoScalerServerNodeGroup.deleteNodeGroup() error = %v", err)
		}
	}
}

func (m *baseTest) KubeClient() (kubernetes.Interface, error) {
	return nil, nil
}

func (m *baseTest) NodeManagerClient() (managednodeClientset.Interface, error) {
	return nil, nil
}

func (m *baseTest) ApiExtentionClient() (apiextension.Interface, error) {
	return nil, nil
}

func (m *baseTest) PodList(nodeName string, podFilter types.PodFilterFunc) ([]apiv1.Pod, error) {
	return nil, nil
}

func (m *baseTest) NodeList() (*apiv1.NodeList, error) {
	node := apiv1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: testNodeName,
			UID:  testCRDUID,
			Annotations: map[string]string{
				constantes.AnnotationNodeGroupName:        testGroupID,
				constantes.AnnotationNodeIndex:            "0",
				constantes.AnnotationInstanceID:           testVMUUID,
				constantes.AnnotationNodeAutoProvisionned: "true",
				constantes.AnnotationScaleDownDisabled:    "false",
				constantes.AnnotationNodeManaged:          "false",
			},
		},
	}

	return &apiv1.NodeList{
		Items: []apiv1.Node{
			node,
		},
	}, nil
}

func (m *baseTest) UncordonNode(nodeName string) error {
	return nil
}

func (m *baseTest) CordonNode(nodeName string) error {
	return nil
}

func (m *baseTest) SetProviderID(nodeName, providerID string) error {
	return nil
}

func (m *baseTest) MarkDrainNode(nodeName string) error {
	return nil
}

func (m *baseTest) DrainNode(nodeName string, ignoreDaemonSet, deleteLocalData bool) error {
	return nil
}

func (m *baseTest) GetNode(nodeName string) (*apiv1.Node, error) {
	node := &apiv1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
			UID:  testCRDUID,
			Annotations: map[string]string{
				constantes.AnnotationNodeGroupName:        testGroupID,
				constantes.AnnotationNodeIndex:            "0",
				constantes.AnnotationInstanceID:           findInstanceID(m.testConfig, nodeName),
				constantes.AnnotationNodeAutoProvisionned: "true",
				constantes.AnnotationScaleDownDisabled:    "false",
				constantes.AnnotationNodeManaged:          "false",
			},
		},
	}

	return node, nil
}

func (m *baseTest) DeleteNode(nodeName string) error {
	return nil
}

func (m *baseTest) AnnoteNode(nodeName string, annotations map[string]string) error {
	return nil
}

func (m *baseTest) LabelNode(nodeName string, labels map[string]string) error {
	return nil
}

func (m *baseTest) TaintNode(nodeName string, taints ...apiv1.Taint) error {
	return nil
}

func (m *baseTest) WaitNodeToBeReady(nodeName string) error {
	return nil
}

func (m *baseTest) newTestNodeNamedWithState(nodeName string, state AutoScalerServerNodeState) (*autoScalerServerNodeGroupTest, *AutoScalerServerNode, error) {

	if ng, err := m.newTestNodeGroup(); err == nil {
		vm := ng.createTestNode(nodeName, state)

		return ng, vm, err
	} else {
		return nil, nil, err
	}
}

func (m *baseTest) newTestNode(name ...string) (*autoScalerServerNodeGroupTest, *AutoScalerServerNode, error) {
	nodeName := testNodeName

	if len(name) > 0 {
		nodeName = name[0]
	}

	return m.newTestNodeNamedWithState(nodeName, AutoScalerServerNodeStateNotCreated)
}

func (m *baseTest) newTestNodeGroup() (*autoScalerServerNodeGroupTest, error) {
	config, err := m.newTestConfig()

	if err == nil {
		if machine, ok := config.Machines[config.DefaultMachineType]; ok {
			ng := &autoScalerServerNodeGroupTest{
				baseTest: baseTest{
					t:          m.t,
					testConfig: m.testConfig,
				},
				AutoScalerServerNodeGroup: AutoScalerServerNodeGroup{
					AutoProvision:              true,
					ServiceIdentifier:          config.ServiceIdentifier,
					NodeGroupIdentifier:        testGroupID,
					ProvisionnedNodeNamePrefix: config.ProvisionnedNodeNamePrefix,
					ManagedNodeNamePrefix:      config.ManagedNodeNamePrefix,
					ControlPlaneNamePrefix:     config.ControlPlaneNamePrefix,
					Status:                     NodegroupCreated,
					MinNodeSize:                config.MinNode,
					MaxNodeSize:                config.MaxNode,
					SystemLabels:               types.KubernetesLabel{},
					Nodes:                      make(map[string]*AutoScalerServerNode),
					RunningNodes:               make(map[int]ServerNodeState),
					pendingNodes:               make(map[string]*AutoScalerServerNode),
					configuration:              config,
					Machine:                    machine,
					NodeLabels:                 config.NodeLabels,
				},
			}

			return ng, err
		}

		m.t.Fatalf("Unable to find machine definition for type: %s", config.DefaultMachineType)
	}

	return nil, err
}

func (m *baseTest) getConfFile() string {
	if config := os.Getenv("TEST_CONFIG"); config != "" {
		return config
	}

	return "../test/config.json"
}

func (m *baseTest) newTestConfig() (*types.AutoScalerServerConfig, error) {
	var config types.AutoScalerServerConfig

	if configStr, err := os.ReadFile(m.getConfFile()); err != nil {
		return nil, err
	} else {
		if err = json.Unmarshal(configStr, &config); err == nil {
			m.testConfig = config.GetVSphereConfiguration(testGroupID)
			m.testConfig.TestMode = true
			config.SSH.TestMode = true
		}

		return &config, err
	}
}

func (m *baseTest) ssh() {
	config, err := m.newTestConfig()

	if assert.NoError(m.t, err) {
		if _, err = utils.Sudo(config.SSH, "127.0.0.1", 1, "ls"); err != nil {
			m.t.Errorf("SSH error = %v", err)
		}
	}
}

func findInstanceID(vsphere *vsphere.Configuration, nodeName string) string {
	if vmUUID, err := vsphere.UUID(nodeName); err == nil {
		return vmUUID
	}

	return testVMUUID
}

func Test_SSH(t *testing.T) {
	createTestNodegroup(t).ssh()
}

func createTestNodegroup(t *testing.T) *nodegroupTest {
	return &nodegroupTest{
		baseTest: baseTest{
			t: t,
		},
	}
}

func TestNodeGroup_launchVM(t *testing.T) {
	createTestNodegroup(t).launchVM()
}

func TestNodeGroup_startVM(t *testing.T) {
	createTestNodegroup(t).startVM()
}

func TestNodeGroup_stopVM(t *testing.T) {
	createTestNodegroup(t).stopVM()
}

func TestNodeGroup_deleteVM(t *testing.T) {
	createTestNodegroup(t).deleteVM()
}

func TestNodeGroup_statusVM(t *testing.T) {
	createTestNodegroup(t).statusVM()
}

func TestNodeGroupGroup_addNode(t *testing.T) {
	createTestNodegroup(t).addNode()
}

func TestNodeGroupGroup_deleteNode(t *testing.T) {
	createTestNodegroup(t).deleteNode()
}

func TestNodeGroupGroup_deleteNodeGroup(t *testing.T) {
	createTestNodegroup(t).deleteNodeGroup()
}
