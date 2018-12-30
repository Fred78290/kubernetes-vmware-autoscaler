package server

import (
	"encoding/json"
	"io/ioutil"
	"testing"

	"github.com/Fred78290/kubernetes-vmware-autoscaler/types"
	"github.com/stretchr/testify/assert"
)

const (
	kubeconfig = "/etc/kubernetes/config"
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
	vm      vm
}

var testNode = []nodeTest{
	nodeTest{
		name:    "Test Node VM",
		wantErr: false,
		vm: vm{
			name:   testNodeName,
			memory: 2048,
			cpu:    2,
			disk:   5120,
			address: []string{
				"127.0.0.1",
			},
		},
	},
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
	config, err := newTestConfig()

	if assert.NoError(t, err) {
		for _, tt := range testNode {
			t.Run(tt.name, func(t *testing.T) {
				vm := &AutoScalerServerNode{
					NodeName:         tt.vm.name,
					Memory:           tt.vm.memory,
					CPU:              tt.vm.cpu,
					Disk:             tt.vm.disk,
					Addresses:        tt.vm.address,
					State:            AutoScalerServerNodeStateNotCreated,
					AutoProvisionned: true,
				}

				nodeLabels := map[string]string{
					"monitor":  "true",
					"database": "true",
				}

				extras := &nodeCreationExtra{
					kubeHost:      config.KubeAdm.Address,
					kubeToken:     config.KubeAdm.Token,
					kubeCACert:    config.KubeAdm.CACert,
					kubeExtraArgs: config.KubeAdm.ExtraArguments,
					kubeConfig:    config.KubeCtlConfig,
					image:         config.Image,
					cloudInit:     config.CloudInit,
					syncFolders:   config.SyncFolders,
					nodegroupID:   testGroupID,
					nodeLabels:    nodeLabels,
					systemLabels:  make(map[string]string),
					vmprovision:   config.VMProvision,
				}

				if err := vm.launchVM(extras); (err != nil) != tt.wantErr {
					t.Errorf("AutoScalerNode.launchVM() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	}
}

func Test_AutoScalerNode_startVM(t *testing.T) {
	for _, tt := range testNode {
		t.Run(tt.name, func(t *testing.T) {
			vm := &AutoScalerServerNode{
				NodeName:         tt.vm.name,
				Memory:           tt.vm.memory,
				CPU:              tt.vm.cpu,
				Disk:             tt.vm.disk,
				Addresses:        tt.vm.address,
				State:            AutoScalerServerNodeStateNotCreated,
				AutoProvisionned: true,
			}
			if err := vm.startVM(kubeconfig); (err != nil) != tt.wantErr {
				t.Errorf("AutoScalerNode.startVM() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_AutoScalerNode_stopVM(t *testing.T) {
	for _, tt := range testNode {
		t.Run(tt.name, func(t *testing.T) {
			vm := &AutoScalerServerNode{
				NodeName:         tt.vm.name,
				Memory:           tt.vm.memory,
				CPU:              tt.vm.cpu,
				Disk:             tt.vm.disk,
				Addresses:        tt.vm.address,
				State:            AutoScalerServerNodeStateNotCreated,
				AutoProvisionned: true,
			}
			if err := vm.stopVM(kubeconfig); (err != nil) != tt.wantErr {
				t.Errorf("AutoScalerNode.stopVM() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_AutoScalerNode_deleteVM(t *testing.T) {
	for _, tt := range testNode {
		t.Run(tt.name, func(t *testing.T) {
			vm := &AutoScalerServerNode{
				NodeName:         tt.vm.name,
				Memory:           tt.vm.memory,
				CPU:              tt.vm.cpu,
				Disk:             tt.vm.disk,
				Addresses:        tt.vm.address,
				State:            AutoScalerServerNodeStateNotCreated,
				AutoProvisionned: true,
			}
			if err := vm.deleteVM(kubeconfig); (err != nil) != tt.wantErr {
				t.Errorf("AutoScalerNode.deleteVM() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_AutoScalerNode_statusVM(t *testing.T) {
	for _, tt := range testNode {
		t.Run(tt.name, func(t *testing.T) {
			vm := &AutoScalerServerNode{
				NodeName:         tt.vm.name,
				Memory:           tt.vm.memory,
				CPU:              tt.vm.cpu,
				Disk:             tt.vm.disk,
				Addresses:        tt.vm.address,
				State:            AutoScalerServerNodeStateNotCreated,
				AutoProvisionned: true,
			}
			got, err := vm.statusVM()
			if (err != nil) != tt.wantErr {
				t.Errorf("AutoScalerNode.statusVM() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != AutoScalerServerNodeStateRunning {
				t.Errorf("AutoScalerNode.statusVM() = %v, want %v", got, AutoScalerServerNodeStateRunning)
			}
		})
	}
}

func Test_AutoScalerNodeGroup_addNode(t *testing.T) {
	config, err := newTestConfig()

	if assert.NoError(t, err) {
		extras := &nodeCreationExtra{
			kubeHost:      config.KubeAdm.Address,
			kubeToken:     config.KubeAdm.Token,
			kubeCACert:    config.KubeAdm.CACert,
			kubeExtraArgs: config.KubeAdm.ExtraArguments,
			kubeConfig:    config.KubeCtlConfig,
			image:         config.Image,
			cloudInit:     config.CloudInit,
			syncFolders:   config.SyncFolders,
			nodegroupID:   testGroupID,
			nodeLabels:    testNodeGroup.NodeLabels,
			systemLabels:  testNodeGroup.SystemLabels,
			vmprovision:   config.VMProvision,
		}

		tests := []struct {
			name    string
			delta   int
			ng      *AutoScalerServerNodeGroup
			wantErr bool
		}{
			{
				name:    "addNode",
				delta:   1,
				wantErr: false,
				ng:      &testNodeGroup,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if err := tt.ng.addNodes(tt.delta, extras); (err != nil) != tt.wantErr {
					t.Errorf("AutoScalerServerNodeGroup.addNode() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	}
}

func Test_AutoScalerNodeGroup_deleteNode(t *testing.T) {
	ng := &AutoScalerServerNodeGroup{
		ServiceIdentifier:   testProviderID,
		NodeGroupIdentifier: testGroupID,
		Machine: &types.MachineCharacteristic{
			Memory: 4096,
			Vcpu:   4,
			Disk:   5120,
		},
		Status:       NodegroupNotCreated,
		MinNodeSize:  0,
		MaxNodeSize:  5,
		PendingNodes: make(map[string]*AutoScalerServerNode),
		Nodes: map[string]*AutoScalerServerNode{
			testNodeName: &AutoScalerServerNode{
				NodeName:         testNodeName,
				Memory:           4096,
				CPU:              4,
				Disk:             5120,
				Addresses:        []string{},
				State:            AutoScalerServerNodeStateNotCreated,
				AutoProvisionned: true,
			},
		},
	}

	tests := []struct {
		name     string
		delta    int
		nodeName string
		ng       *AutoScalerServerNodeGroup
		wantErr  bool
	}{
		{
			name:     "deleteNode",
			delta:    1,
			wantErr:  false,
			nodeName: testNodeName,
			ng:       ng,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.ng.deleteNodeByName(kubeconfig, tt.nodeName); (err != nil) != tt.wantErr {
				t.Errorf("AutoScalerServerNodeGroup.deleteNode() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_AutoScalerNodeGroup_deleteNodeGroup(t *testing.T) {
	ng := &AutoScalerServerNodeGroup{
		ServiceIdentifier:   testProviderID,
		NodeGroupIdentifier: testGroupID,
		Machine: &types.MachineCharacteristic{
			Memory: 4096,
			Vcpu:   4,
			Disk:   5120,
		},
		Status:       NodegroupNotCreated,
		MinNodeSize:  0,
		MaxNodeSize:  5,
		PendingNodes: make(map[string]*AutoScalerServerNode),
		Nodes: map[string]*AutoScalerServerNode{
			testNodeName: &AutoScalerServerNode{
				NodeName:         testNodeName,
				Memory:           4096,
				CPU:              4,
				Disk:             5120,
				Addresses:        []string{},
				State:            AutoScalerServerNodeStateNotCreated,
				AutoProvisionned: true,
			},
		},
	}

	tests := []struct {
		name     string
		delta    int
		nodeName string
		ng       *AutoScalerServerNodeGroup
		wantErr  bool
	}{
		{
			name:    "deleteNodeGroup",
			delta:   1,
			wantErr: false,
			ng:      ng,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.ng.deleteNodeGroup(kubeconfig); (err != nil) != tt.wantErr {
				t.Errorf("AutoScalerServerNodeGroup.deleteNodeGroup() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
