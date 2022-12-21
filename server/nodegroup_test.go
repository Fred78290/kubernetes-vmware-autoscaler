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

type nodegroupTest struct {
	t *testing.T
}

func (m *nodegroupTest) launchVM() {
	_, ng, testNode, kubeClient, err := newTestNode(launchVMName)

	if assert.NoError(m.t, err) {
		if err := testNode.launchVM(kubeClient, ng.NodeLabels, ng.SystemLabels); err != nil {
			m.t.Errorf("AutoScalerNode.launchVM() error = %v", err)
		}
	}
}

func (m *nodegroupTest) startVM() {
	_, _, testNode, kubeClient, err := newTestNode(launchVMName)

	if assert.NoError(m.t, err) {
		if err := testNode.startVM(kubeClient); err != nil {
			m.t.Errorf("AutoScalerNode.startVM() error = %v", err)
		}
	}
}

func (m *nodegroupTest) stopVM() {
	_, _, testNode, kubeClient, err := newTestNode(launchVMName)

	if assert.NoError(m.t, err) {
		if err := testNode.stopVM(kubeClient); err != nil {
			m.t.Errorf("AutoScalerNode.stopVM() error = %v", err)
		}
	}
}

func (m *nodegroupTest) deleteVM() {
	_, _, testNode, kubeClient, err := newTestNode(launchVMName)

	if assert.NoError(m.t, err) {
		if err := testNode.deleteVM(kubeClient); err != nil {
			m.t.Errorf("AutoScalerNode.deleteVM() error = %v", err)
		}
	}
}

func (m *nodegroupTest) statusVM() {
	_, _, testNode, _, err := newTestNode(launchVMName)

	if assert.NoError(m.t, err) {
		if got, err := testNode.statusVM(); err != nil {
			m.t.Errorf("AutoScalerNode.statusVM() error = %v", err)
		} else if got != AutoScalerServerNodeStateRunning {
			m.t.Errorf("AutoScalerNode.statusVM() = %v, want %v", got, AutoScalerServerNodeStateRunning)
		}
	}
}

func (m *nodegroupTest) addNode() {
	_, ng, kubeClient, err := newTestNodeGroup()

	if assert.NoError(m.t, err) {
		if _, err := ng.addNodes(kubeClient, 1); err != nil {
			m.t.Errorf("AutoScalerServerNodeGroup.addNode() error = %v", err)
		}
	}
}

func (m *nodegroupTest) deleteNode() {
	_, ng, testNode, kubeClient, err := newTestNode(launchVMName)

	if assert.NoError(m.t, err) {
		if err := ng.deleteNodeByName(kubeClient, testNode.NodeName); err != nil {
			m.t.Errorf("AutoScalerServerNodeGroup.deleteNode() error = %vv", err)
		}
	}
}

func (m *nodegroupTest) deleteNodeGroup() {
	_, ng, kubeClient, err := newTestNodeGroup()

	if assert.NoError(m.t, err) {
		if err := ng.deleteNodeGroup(kubeClient); err != nil {
			m.t.Errorf("AutoScalerServerNodeGroup.deleteNodeGroup() error = %v", err)
		}
	}
}

type mockupClientGenerator struct {
	vsphere *vsphere.Configuration
}

func (m mockupClientGenerator) fixAnnotation(node *apiv1.Node) {
	if vmUUID, err := m.vsphere.UUID(node.Name); err == nil {
		node.Annotations[constantes.AnnotationInstanceID] = vmUUID
	}
}

func (m mockupClientGenerator) KubeClient() (kubernetes.Interface, error) {
	return nil, nil
}

func (m mockupClientGenerator) NodeManagerClient() (managednodeClientset.Interface, error) {
	return nil, nil
}

func (m mockupClientGenerator) ApiExtentionClient() (apiextension.Interface, error) {
	return nil, nil
}

func (m mockupClientGenerator) PodList(nodeName string, podFilter types.PodFilterFunc) ([]apiv1.Pod, error) {
	return nil, nil
}

func (m mockupClientGenerator) NodeList() (*apiv1.NodeList, error) {
	node := apiv1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: testNodeName,
			UID:  testCRDUID,
			Annotations: map[string]string{
				constantes.AnnotationNodeGroupName:        testGroupID,
				constantes.AnnotationNodeIndex:            "0",
				constantes.AnnotationInstanceID:           testVMUUID,
				constantes.AnnotationNodeAutoProvisionned: "true",
			},
		},
	}

	m.fixAnnotation(&node)

	return &apiv1.NodeList{
		Items: []apiv1.Node{
			node,
		},
	}, nil
}

func (m mockupClientGenerator) UncordonNode(nodeName string) error {
	return nil
}

func (m mockupClientGenerator) CordonNode(nodeName string) error {
	return nil
}

func (m mockupClientGenerator) SetProviderID(nodeName, providerID string) error {
	return nil
}

func (m mockupClientGenerator) MarkDrainNode(nodeName string) error {
	return nil
}

func (m mockupClientGenerator) DrainNode(nodeName string, ignoreDaemonSet, deleteLocalData bool) error {
	return nil
}

func (m mockupClientGenerator) GetNode(nodeName string) (*apiv1.Node, error) {
	node := &apiv1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: testNodeName,
			UID:  testCRDUID,
			Annotations: map[string]string{
				constantes.AnnotationNodeGroupName:        testGroupID,
				constantes.AnnotationNodeIndex:            "0",
				constantes.AnnotationInstanceID:           testVMUUID,
				constantes.AnnotationNodeAutoProvisionned: "true",
			},
		},
	}

	m.fixAnnotation(node)

	return node, nil
}

func (m mockupClientGenerator) DeleteNode(nodeName string) error {
	return nil
}

func (m mockupClientGenerator) AnnoteNode(nodeName string, annotations map[string]string) error {
	return nil
}

func (m mockupClientGenerator) LabelNode(nodeName string, labels map[string]string) error {
	return nil
}

func (m mockupClientGenerator) TaintNode(nodeName string, taints ...apiv1.Taint) error {
	return nil
}

func (m mockupClientGenerator) WaitNodeToBeReady(nodeName string, timeToWaitInSeconds int) error {
	return nil
}

func createTestNode(ng *AutoScalerServerNodeGroup, nodeName string, desiredState ...AutoScalerServerNodeState) *AutoScalerServerNode {
	var state AutoScalerServerNodeState = AutoScalerServerNodeStateNotCreated

	if len(desiredState) > 0 {
		state = desiredState[0]
	}

	config := ng.configuration.GetVSphereConfiguration(testGroupID)
	config.UseSimulator = true

	node := &AutoScalerServerNode{
		NodeGroupID:   testGroupID,
		NodeName:      nodeName,
		VMUUID:        testVMUUID,
		CRDUID:        testCRDUID,
		Memory:        2048,
		CPU:           2,
		Disk:          5120,
		IPAddress:     "127.0.0.1",
		State:         state,
		NodeType:      AutoScalerServerNodeAutoscaled,
		NodeIndex:     1,
		VSphereConfig: config,
		serverConfig:  ng.configuration,
	}

	if vmuuid := node.findInstanceUUID(); len(vmuuid) > 0 {
		node.VMUUID = vmuuid
	}

	return node
}

func newTestNode(name ...string) (*types.AutoScalerServerConfig, *AutoScalerServerNodeGroup, *AutoScalerServerNode, types.ClientGenerator, error) {
	nodeName := testNodeName
	config, ng, kubeclient, err := newTestNodeGroup()

	if len(name) > 0 {
		nodeName = name[0]
	}

	if err == nil {
		vm := createTestNode(ng, nodeName)

		ng.Nodes[nodeName] = vm
		ng.RunningNodes[1] = ServerNodeStateRunning

		return config, ng, vm, kubeclient, err
	}

	return config, ng, nil, nil, err
}

func newTestNodeGroup() (*types.AutoScalerServerConfig, *AutoScalerServerNodeGroup, types.ClientGenerator, error) {
	config, kubeclient, err := newTestConfig()

	if err == nil {
		ng := &AutoScalerServerNodeGroup{
			AutoProvision:              true,
			ServiceIdentifier:          testServiceIdentifier,
			NodeGroupIdentifier:        testGroupID,
			ProvisionnedNodeNamePrefix: "autoscaled",
			ManagedNodeNamePrefix:      "worker",
			ControlPlaneNamePrefix:     "master",
			Status:                     NodegroupCreated,
			MinNodeSize:                0,
			MaxNodeSize:                5,
			SystemLabels:               types.KubernetesLabel{},
			Nodes:                      make(map[string]*AutoScalerServerNode),
			RunningNodes:               make(map[int]ServerNodeState),
			pendingNodes:               make(map[string]*AutoScalerServerNode),
			configuration:              config,
			Machine: &types.MachineCharacteristic{
				Memory: 4096,
				Vcpu:   4,
				Disk:   5120,
			},
			NodeLabels: types.KubernetesLabel{
				"monitor":  "true",
				"database": "true",
			},
		}

		return config, ng, kubeclient, err
	}

	return nil, nil, nil, err
}

func newTestConfig() (*types.AutoScalerServerConfig, types.ClientGenerator, error) {
	var config types.AutoScalerServerConfig

	if configStr, err := os.ReadFile("../test/config.json"); err != nil {
		return nil, nil, err
	} else {
		err = json.Unmarshal(configStr, &config)

		if err != nil {
			return nil, nil, err
		}

		vsphere := config.GetVSphereConfiguration(testGroupID)

		vsphere.UseSimulator = true
		config.SSH.UseSimulator = true

		kubeClient := mockupClientGenerator{
			vsphere: vsphere,
		}

		return &config, kubeClient, nil
	}
}

func Test_SSH(t *testing.T) {
	config, _, err := newTestConfig()

	if assert.NoError(t, err) {
		t.Run("Launch VM", func(t *testing.T) {
			if _, err = utils.Sudo(config.SSH, "127.0.0.1", 1, "ls"); err != nil {
				t.Errorf("SSH error = %v", err)
			}
		})
	}

}

func TestNodeGroup_launchVM(t *testing.T) {
	test := nodegroupTest{t: t}

	test.launchVM()
}

func TestNodeGroup_startVM(t *testing.T) {
	test := nodegroupTest{t: t}

	test.startVM()
}

func TestNodeGroup_stopVM(t *testing.T) {
	test := nodegroupTest{t: t}

	test.stopVM()
}

func TestNodeGroup_deleteVM(t *testing.T) {
	test := nodegroupTest{t: t}

	test.deleteVM()
}

func TestNodeGroup_statusVM(t *testing.T) {
	test := nodegroupTest{t: t}

	test.statusVM()
}

func TestNodeGroupGroup_addNode(t *testing.T) {
	test := nodegroupTest{t: t}

	test.addNode()
}

func TestNodeGroupGroup_deleteNode(t *testing.T) {
	test := nodegroupTest{t: t}

	test.deleteNode()
}

func TestNodeGroupGroup_deleteNodeGroup(t *testing.T) {
	test := nodegroupTest{t: t}

	test.deleteNodeGroup()
}
