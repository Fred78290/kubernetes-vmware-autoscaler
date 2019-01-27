package server

import (
	"encoding/json"
	"io/ioutil"
	"testing"

	"github.com/Fred78290/kubernetes-vmware-autoscaler/types"
	"github.com/stretchr/testify/assert"
)

type arguments struct {
	kubeHost      string
	kubeToken     string
	kubeCACert    string
	kubeExtraArgs []string
	image         string
	cloudInit     *map[string]interface{}
	mountPoints   *map[string]string
}

type vm struct {
	name    string
	memory  int
	cpu     int
	disk    int
	address []string
}

type nodeTest struct {
	name    string
	wantErr bool
	node    *AutoScalerServerNode
}

func createTestNode(ng *AutoScalerServerNodeGroup) *AutoScalerServerNode {
	return &AutoScalerServerNode{
		ProviderID:  ng.providerIDForNode(testNodeName),
		NodeGroupID: testGroupID,
		NodeName:    testNodeName,
		Memory:      2048,
		CPU:         2,
		Disk:        5120,
		Addresses: []string{
			"127.0.0.1",
		},
		State:            AutoScalerServerNodeStateNotCreated,
		AutoProvisionned: true,
		VSphereConfig:    ng.configuration.GetVSphereConfiguration(testGroupID),
		serverConfig:     ng.configuration,
	}
}

func newTestNode() (*types.AutoScalerServerConfig, *AutoScalerServerNodeGroup, *AutoScalerServerNode, error) {
	config, ng, err := newTestNodeGroup()

	if err == nil {
		vm := createTestNode(ng)

		ng.Nodes[testGroupID] = vm

		return config, ng, vm, err
	}

	return config, ng, nil, err
}

func newTestNodeGroup() (*types.AutoScalerServerConfig, *AutoScalerServerNodeGroup, error) {
	config, err := newTestConfig()

	if err == nil {
		ng := &AutoScalerServerNodeGroup{
			ServiceIdentifier:   testProviderID,
			NodeGroupIdentifier: testGroupID,
			Machine: &types.MachineCharacteristic{
				Memory: 4096,
				Vcpu:   4,
				Disk:   5120,
			},
			Status:      NodegroupNotCreated,
			MinNodeSize: 0,
			MaxNodeSize: 5,
			NodeLabels: KubernetesLabel{
				"monitor":  "true",
				"database": "true",
			},
			SystemLabels:  KubernetesLabel{},
			Nodes:         make(map[string]*AutoScalerServerNode),
			pendingNodes:  make(map[string]*AutoScalerServerNode),
			configuration: config,
		}

		return config, ng, err
	}

	return nil, nil, err
}

func newTestConfig() (*types.AutoScalerServerConfig, error) {
	var config types.AutoScalerServerConfig

	configStr, err := ioutil.ReadFile("./masterkube/config/config.json")
	err = json.Unmarshal(configStr, &config)

	if err != nil {
		return nil, err
	}

	return &config, nil
}

func Test_AutoScalerNode_launchVM(t *testing.T) {
	_, ng, testNode, err := newTestNode()

	if assert.NoError(t, err) {
		t.Run("Launch VM", func(t *testing.T) {
			if err := testNode.launchVM(ng.NodeLabels, ng.SystemLabels); err != nil {
				t.Errorf("AutoScalerNode.launchVM() error = %v", err)
			}
		})
	}
}

func Test_AutoScalerNode_startVM(t *testing.T) {
	_, _, testNode, err := newTestNode()

	if assert.NoError(t, err) {
		t.Run("Start VM", func(t *testing.T) {
			if err := testNode.startVM(); err != nil {
				t.Errorf("AutoScalerNode.startVM() error = %v", err)
			}
		})
	}
}

func Test_AutoScalerNode_stopVM(t *testing.T) {
	_, _, testNode, err := newTestNode()

	if assert.NoError(t, err) {
		t.Run("Stop VM", func(t *testing.T) {
			if err := testNode.stopVM(); err != nil {
				t.Errorf("AutoScalerNode.stopVM() error = %v", err)
			}
		})
	}
}

func Test_AutoScalerNode_deleteVM(t *testing.T) {
	_, _, testNode, err := newTestNode()

	if assert.NoError(t, err) {
		t.Run("Delete VM", func(t *testing.T) {
			if err := testNode.deleteVM(); err != nil {
				t.Errorf("AutoScalerNode.deleteVM() error = %v", err)
			}
		})
	}
}

func Test_AutoScalerNode_statusVM(t *testing.T) {
	_, _, testNode, err := newTestNode()

	if assert.NoError(t, err) {
		t.Run("Status VM", func(t *testing.T) {
			if got, err := testNode.statusVM(); err != nil {
				t.Errorf("AutoScalerNode.statusVM() error = %v", err)
			} else if got != AutoScalerServerNodeStateRunning {
				t.Errorf("AutoScalerNode.statusVM() = %v, want %v", got, AutoScalerServerNodeStateRunning)
			}
		})
	}
}

func Test_AutoScalerNodeGroup_addNode(t *testing.T) {
	_, ng, err := newTestNodeGroup()

	if assert.NoError(t, err) {
		t.Run("addNode", func(t *testing.T) {
			if err := ng.addNodes(1); err != nil {
				t.Errorf("AutoScalerServerNodeGroup.addNode() error = %v", err)
			}
		})
	}
}

func Test_AutoScalerNodeGroup_deleteNode(t *testing.T) {
	_, ng, testNode, err := newTestNode()

	if assert.NoError(t, err) {
		t.Run("Delete VM", func(t *testing.T) {
			if err := ng.deleteNodeByName(testNode.NodeName); err != nil {
				t.Errorf("AutoScalerServerNodeGroup.deleteNode() error = %vv", err)
			}
		})
	}
}

func Test_AutoScalerNodeGroup_deleteNodeGroup(t *testing.T) {
	_, ng, _, err := newTestNode()

	if assert.NoError(t, err) {

		t.Run("Delete node group", func(t *testing.T) {
			if err := ng.deleteNodeGroup(); err != nil {
				t.Errorf("AutoScalerServerNodeGroup.deleteNodeGroup() error = %v", err)
			}
		})
	}
}
