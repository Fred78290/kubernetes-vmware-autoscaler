package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	apigrpc "github.com/Fred78290/kubernetes-vmware-autoscaler/grpc"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider"
)

const (
	testProviderID = "AutoScaler"
	testGroupID    = "ca-grpc-AutoScaler"
	testNodeName   = "ca-grpc-AutoScaler-vm-00"
)

var testNodeGroup = AutoScalerServerNodeGroup{
	NodeGroupIdentifier: testGroupID,
	Machine: &MachineCharacteristic{
		Memory: 4096,
		Vcpu:   4,
		Disk:   5120,
	},
	Status:       NodegroupNotCreated,
	MinNodeSize:  0,
	MaxNodeSize:  5,
	PendingNodes: make(map[string]*AutoScalerServerNode),
	Nodes:        make(map[string]*AutoScalerServerNode),
	NodeLabels: map[string]string{
		"monitor":  "true",
		"database": "true",
	},
}

func newTestServer(nodeGroup *AutoScalerServerNodeGroup) (*AutoScalerServerApp, context.Context, error) {

	var config AutoScalerServerConfig

	configStr, _ := ioutil.ReadFile("./masterkube/config/config.json")

	err := json.Unmarshal(configStr, &config)

	if err != nil {
		return nil, nil, err
	}

	s := &AutoScalerServerApp{
		ResourceLimiter: &ResourceLimiter{
			map[string]int64{cloudprovider.ResourceNameCores: 1, cloudprovider.ResourceNameMemory: 10000000},
			map[string]int64{cloudprovider.ResourceNameCores: 5, cloudprovider.ResourceNameMemory: 100000000},
		},
		Groups:        map[string]*AutoScalerServerNodeGroup{},
		Configuration: &config,
		KubeAdmConfiguration: &apigrpc.KubeAdmConfig{
			KubeAdmAddress:        config.KubeAdm.Address,
			KubeAdmToken:          config.KubeAdm.Token,
			KubeAdmCACert:         config.KubeAdm.CACert,
			KubeAdmExtraArguments: config.KubeAdm.ExtraArguments,
		},
	}

	if nodeGroup != nil {
		s.Groups[nodeGroup.NodeGroupIdentifier] = nodeGroup
	}

	return s, nil, nil
}

func extractNodeGroup(nodeGroups []*apigrpc.NodeGroup) []string {
	r := make([]string, len(nodeGroups))

	for i, n := range nodeGroups {
		r[i] = n.Id
	}

	return r
}

func TestAutoScalerServer_NodeGroups(t *testing.T) {
	tests := []struct {
		name    string
		request *apigrpc.CloudProviderServiceRequest
		want    []string
		wantErr bool
	}{
		{
			name: "NodeGroups",
			request: &apigrpc.CloudProviderServiceRequest{
				ProviderID: testProviderID,
			},
			want: []string{
				testGroupID,
			},
		},
	}

	ng := AutoScalerServerNodeGroup{
		NodeGroupIdentifier: testGroupID,
		Machine: &MachineCharacteristic{
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

	s, ctx, err := newTestServer(&ng)

	if assert.NoError(t, err) {
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got, err := s.NodeGroups(ctx, tt.request)

				if (err != nil) != tt.wantErr {
					t.Errorf("AutoScalerServerApp.NodeGroups() error = %v, wantErr %v", err, tt.wantErr)
				} else if !reflect.DeepEqual(extractNodeGroup(got.GetNodeGroups()), tt.want) {
					t.Errorf("AutoScalerServerApp.NodeGroups() = %v, want %v", extractNodeGroup(got.GetNodeGroups()), tt.want)
				}
			})
		}
	}
}

func TestAutoScalerServer_NodeGroupForNode(t *testing.T) {
	tests := []struct {
		name    string
		request *apigrpc.NodeGroupForNodeRequest
		want    string
		wantErr bool
	}{
		{
			name: "NodeGroupForNode",
			want: testGroupID,
			request: &apigrpc.NodeGroupForNodeRequest{
				ProviderID: testProviderID,
				Node: toJSON(
					apiv1.Node{
						Spec: apiv1.NodeSpec{
							ProviderID: fmt.Sprintf("%s://%s/object?type=node&name=%s", testProviderID, testGroupID, testNodeName),
						},
					},
				),
			},
		},
	}

	ng := AutoScalerServerNodeGroup{
		NodeGroupIdentifier: testGroupID,
		Machine: &MachineCharacteristic{
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

	s, ctx, err := newTestServer(&ng)

	if assert.NoError(t, err) {
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got, err := s.NodeGroupForNode(ctx, tt.request)

				if (err != nil) != tt.wantErr {
					t.Errorf("AutoScalerServerApp.NodeGroupForNode() error = %v, wantErr %v", err, tt.wantErr)
				} else if got.GetError() != nil {
					t.Errorf("AutoScalerServerApp.NodeGroupForNode() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
				} else if !reflect.DeepEqual(got.GetNodeGroup().GetId(), tt.want) {
					t.Errorf("AutoScalerServerApp.NodeGroupForNode() = %v, want %v", got.GetNodeGroup().GetId(), tt.want)
				}
			})
		}
	}
}

func TestAutoScalerServer_Pricing(t *testing.T) {
	tests := []struct {
		name    string
		request *apigrpc.CloudProviderServiceRequest
		want    string
		wantErr bool
	}{
		{
			name: "Pricing",
			want: testProviderID,
			request: &apigrpc.CloudProviderServiceRequest{
				ProviderID: testProviderID,
			},
		},
	}

	s, ctx, err := newTestServer(&testNodeGroup)

	if assert.NoError(t, err) {
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got, err := s.Pricing(ctx, tt.request)

				if (err != nil) != tt.wantErr {
					t.Errorf("AutoScalerServerApp.Pricing() error = %v, wantErr %v", err, tt.wantErr)
				} else if got.GetError() != nil {
					t.Errorf("AutoScalerServerApp.Pricing() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
				} else if !reflect.DeepEqual(got.GetPriceModel().GetId(), tt.want) {
					t.Errorf("AutoScalerServerApp.Pricing() = %v, want %v", got.GetPriceModel().GetId(), tt.want)
				}
			})
		}
	}
}

func extractAvailableMachineTypes(availableMachineTypes *apigrpc.AvailableMachineTypes) []string {
	r := make([]string, len(availableMachineTypes.MachineType))

	for i, m := range availableMachineTypes.MachineType {
		r[i] = m
	}

	return r
}

func TestAutoScalerServer_GetAvailableMachineTypes(t *testing.T) {
	tests := []struct {
		name    string
		request *apigrpc.CloudProviderServiceRequest
		want    []string
		wantErr bool
	}{
		{
			name: "GetAvailableMachineTypes",
			request: &apigrpc.CloudProviderServiceRequest{
				ProviderID: testProviderID,
			},
			want: []string{
				"tiny",
				"medium",
				"large",
				"extra-large",
			},
		},
	}

	s, ctx, err := newTestServer(&testNodeGroup)

	if assert.NoError(t, err) {
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {

				got, err := s.GetAvailableMachineTypes(ctx, tt.request)

				if (err != nil) != tt.wantErr {
					t.Errorf("AutoScalerServerApp.GetAvailableMachineTypes() error = %v, wantErr %v", err, tt.wantErr)
				} else if got.GetError() != nil {
					t.Errorf("AutoScalerServerApp.GetAvailableMachineTypes() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
				} else if !reflect.DeepEqual(extractAvailableMachineTypes(got.GetAvailableMachineTypes()), tt.want) {
					t.Errorf("AutoScalerServerApp.GetAvailableMachineTypes() = %v, want %v", extractAvailableMachineTypes(got.GetAvailableMachineTypes()), tt.want)
				}
			})
		}
	}
}

func TestAutoScalerServer_NewNodeGroup(t *testing.T) {
	tests := []struct {
		name    string
		request *apigrpc.NewNodeGroupRequest
		want    string
		wantErr bool
	}{
		{
			name: "NewNodeGroup",
			request: &apigrpc.NewNodeGroupRequest{
				ProviderID:  testProviderID,
				MachineType: "tiny",
				Labels: map[string]string{
					"database": "true",
					"monitor":  "true",
				},
			},
		},
	}

	s, ctx, err := newTestServer(&testNodeGroup)

	if assert.NoError(t, err) {
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {

				got, err := s.NewNodeGroup(ctx, tt.request)

				if (err != nil) != tt.wantErr {
					t.Errorf("AutoScalerServerApp.NewNodeGroup() error = %v, wantErr %v", err, tt.wantErr)
				} else if got.GetError() != nil {
					t.Errorf("AutoScalerServerApp.NewNodeGroup() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
				} else {
					t.Logf("AutoScalerServerApp.NewNodeGroup() return node group created :%v", got.GetNodeGroup().GetId())
				}
			})
		}
	}
}

func extractResourceLimiter(res *apigrpc.ResourceLimiter) *ResourceLimiter {
	r := &ResourceLimiter{
		MinLimits: res.MinLimits,
		MaxLimits: res.MaxLimits,
	}

	return r
}

func TestAutoScalerServer_GetResourceLimiter(t *testing.T) {
	tests := []struct {
		name    string
		request *apigrpc.CloudProviderServiceRequest
		want    *ResourceLimiter
		wantErr bool
	}{
		{
			name: "GetResourceLimiter",
			want: &ResourceLimiter{
				map[string]int64{cloudprovider.ResourceNameCores: 1, cloudprovider.ResourceNameMemory: 10000000},
				map[string]int64{cloudprovider.ResourceNameCores: 5, cloudprovider.ResourceNameMemory: 100000000},
			},
			request: &apigrpc.CloudProviderServiceRequest{
				ProviderID: testProviderID,
			},
		},
	}

	s, ctx, err := newTestServer(&testNodeGroup)

	if assert.NoError(t, err) {
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got, err := s.GetResourceLimiter(ctx, tt.request)

				if (err != nil) != tt.wantErr {
					t.Errorf("AutoScalerServerApp.GetResourceLimiter() error = %v, wantErr %v", err, tt.wantErr)
				} else if got.GetError() != nil {
					t.Errorf("AutoScalerServerApp.GetResourceLimiter() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
				} else if !reflect.DeepEqual(extractResourceLimiter(got.GetResourceLimiter()), tt.want) {
					t.Errorf("AutoScalerServerApp.GetResourceLimiter() = %v, want %v", got.GetResourceLimiter(), tt.want)
				}
			})
		}
	}
}

func TestAutoScalerServer_Cleanup(t *testing.T) {
	tests := []struct {
		name    string
		request *apigrpc.CloudProviderServiceRequest
		want    *apigrpc.CleanupReply
		wantErr bool
	}{
		{
			name: "Cleanup",
			request: &apigrpc.CloudProviderServiceRequest{
				ProviderID: testProviderID,
			},
		},
	}

	s, ctx, err := newTestServer(&testNodeGroup)

	if assert.NoError(t, err) {
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {

				got, err := s.Cleanup(ctx, tt.request)

				if (err != nil) != tt.wantErr {
					t.Errorf("AutoScalerServerApp.Cleanup() error = %v, wantErr %v", err, tt.wantErr)
				} else if got.GetError() != nil {
					t.Errorf("AutoScalerServerApp.Cleanup() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
				}
			})
		}
	}
}

func TestAutoScalerServer_Refresh(t *testing.T) {
	tests := []struct {
		name    string
		request *apigrpc.CloudProviderServiceRequest
		want    *apigrpc.RefreshReply
		wantErr bool
	}{
		{
			name: "Refresh",
			request: &apigrpc.CloudProviderServiceRequest{
				ProviderID: testProviderID,
			},
		},
	}

	s, ctx, err := newTestServer(&testNodeGroup)

	if assert.NoError(t, err) {
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {

				got, err := s.Refresh(ctx, tt.request)

				if (err != nil) != tt.wantErr {
					t.Errorf("AutoScalerServerApp.Refresh() error = %v, wantErr %v", err, tt.wantErr)
				} else if got.GetError() != nil {
					t.Errorf("AutoScalerServerApp.Refresh() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
				}
			})
		}
	}
}

func TestAutoScalerServer_MaxSize(t *testing.T) {
	tests := []struct {
		name    string
		request *apigrpc.NodeGroupServiceRequest
		want    int32
		wantErr bool
	}{
		{
			name: "TargetSize",
			want: int32(testNodeGroup.MaxNodeSize),
			request: &apigrpc.NodeGroupServiceRequest{
				ProviderID:  testProviderID,
				NodeGroupID: testGroupID,
			},
		},
	}

	s, ctx, err := newTestServer(&testNodeGroup)

	if assert.NoError(t, err) {
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {

				got, err := s.MaxSize(ctx, tt.request)

				if (err != nil) != tt.wantErr {
					t.Errorf("AutoScalerServerApp.MaxSize() error = %v, wantErr %v", err, tt.wantErr)
				} else if got.GetMaxSize() != tt.want {
					t.Errorf("AutoScalerServerApp.MaxSize() = %v, want %v", got.GetMaxSize(), tt.want)
				}
			})
		}
	}
}

func TestAutoScalerServer_MinSize(t *testing.T) {
	tests := []struct {
		name    string
		request *apigrpc.NodeGroupServiceRequest
		want    int32
		wantErr bool
	}{
		{
			name: "MinSize",
			want: int32(testNodeGroup.MinNodeSize),
			request: &apigrpc.NodeGroupServiceRequest{
				ProviderID:  testProviderID,
				NodeGroupID: testGroupID,
			},
		},
	}

	s, ctx, err := newTestServer(&testNodeGroup)

	if assert.NoError(t, err) {
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {

				got, err := s.MinSize(ctx, tt.request)

				if (err != nil) != tt.wantErr {
					t.Errorf("AutoScalerServerApp.MinSize() error = %v, wantErr %v", err, tt.wantErr)
				} else if got.GetMinSize() != tt.want {
					t.Errorf("AutoScalerServerApp.MinSize() = %v, want %v", got.GetMinSize(), tt.want)
				}
			})
		}
	}
}

func TestAutoScalerServer_TargetSize(t *testing.T) {
	tests := []struct {
		name    string
		request *apigrpc.NodeGroupServiceRequest
		want    int32
		wantErr bool
	}{
		{
			name: "TargetSize",
			want: int32(len(testNodeGroup.Nodes)),
			request: &apigrpc.NodeGroupServiceRequest{
				ProviderID:  testProviderID,
				NodeGroupID: testGroupID,
			},
		},
	}

	s, ctx, err := newTestServer(&testNodeGroup)

	if assert.NoError(t, err) {
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {

				got, err := s.TargetSize(ctx, tt.request)

				if (err != nil) != tt.wantErr {
					t.Errorf("AutoScalerServerApp.TargetSize() error = %v, wantErr %v", err, tt.wantErr)
				} else if got.GetError() != nil {
					t.Errorf("AutoScalerServerApp.TargetSize() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
				} else if got.GetTargetSize() != tt.want {
					t.Errorf("AutoScalerServerApp.TargetSize() = %v, want %v", got.GetTargetSize(), tt.want)
				}
			})
		}
	}
}

func TestAutoScalerServer_IncreaseSize(t *testing.T) {
	tests := []struct {
		name    string
		request *apigrpc.IncreaseSizeRequest
		wantErr bool
	}{
		{
			name: "IncreaseSize",
			request: &apigrpc.IncreaseSizeRequest{
				ProviderID:  testProviderID,
				NodeGroupID: testGroupID,
				Delta:       1,
			},
		},
	}

	s, ctx, err := newTestServer(&testNodeGroup)

	if assert.NoError(t, err) {
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {

				got, err := s.IncreaseSize(ctx, tt.request)

				if (err != nil) != tt.wantErr {
					t.Errorf("AutoScalerServerApp.IncreaseSize() error = %v, wantErr %v", err, tt.wantErr)
				} else if got.GetError() != nil {
					t.Errorf("AutoScalerServerApp.IncreaseSize() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
				}
			})
		}
	}
}

func TestAutoScalerServer_DeleteNodes(t *testing.T) {
	tests := []struct {
		name    string
		request *apigrpc.DeleteNodesRequest
		wantErr bool
	}{
		{
			name: "DeleteNodes",
			request: &apigrpc.DeleteNodesRequest{
				ProviderID:  testProviderID,
				NodeGroupID: testGroupID,
				Node: []string{
					toJSON(
						apiv1.Node{
							Spec: apiv1.NodeSpec{
								ProviderID: fmt.Sprintf("%s://%s/object?type=node&name=%s", testProviderID, testGroupID, testNodeName),
							},
						},
					),
				},
			},
		},
	}

	ng := AutoScalerServerNodeGroup{
		NodeGroupIdentifier: testGroupID,
		Machine: &MachineCharacteristic{
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

	s, ctx, err := newTestServer(&ng)

	if assert.NoError(t, err) {
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {

				got, err := s.DeleteNodes(ctx, tt.request)

				if (err != nil) != tt.wantErr {
					t.Errorf("AutoScalerServerApp.DeleteNodes() error = %v, wantErr %v", err, tt.wantErr)
				} else if got.GetError() != nil {
					t.Errorf("AutoScalerServerApp.DeleteNodes() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
				}
			})
		}
	}
}

func TestAutoScalerServer_DecreaseTargetSize(t *testing.T) {
	tests := []struct {
		name    string
		request *apigrpc.DecreaseTargetSizeRequest
		wantErr bool
	}{
		{
			name: "DecreaseTargetSize",
			request: &apigrpc.DecreaseTargetSizeRequest{
				ProviderID:  testProviderID,
				NodeGroupID: testGroupID,
				Delta:       -1,
			},
		},
	}

	ng := AutoScalerServerNodeGroup{
		NodeGroupIdentifier: testGroupID,
		Machine: &MachineCharacteristic{
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

	s, ctx, err := newTestServer(&ng)

	if assert.NoError(t, err) {
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {

				got, err := s.DecreaseTargetSize(ctx, tt.request)

				if (err != nil) != tt.wantErr {
					t.Errorf("AutoScalerServerApp.DecreaseTargetSize() error = %v, wantErr %v", err, tt.wantErr)
				} else if got.GetError() != nil {
					t.Errorf("AutoScalerServerApp.DecreaseTargetSize() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
				}
			})
		}
	}
}

func TestAutoScalerServer_Id(t *testing.T) {
	tests := []struct {
		name    string
		request *apigrpc.NodeGroupServiceRequest
		want    string
		wantErr bool
	}{
		{
			name: "Id",
			want: testGroupID,
			request: &apigrpc.NodeGroupServiceRequest{
				ProviderID:  testProviderID,
				NodeGroupID: testGroupID,
			},
		},
	}

	s, ctx, err := newTestServer(&testNodeGroup)

	if assert.NoError(t, err) {
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {

				got, err := s.Id(ctx, tt.request)

				if (err != nil) != tt.wantErr {
					t.Errorf("AutoScalerServerApp.Id() error = %v, wantErr %v", err, tt.wantErr)
				} else if got.GetResponse() != tt.want {
					t.Errorf("AutoScalerServerApp.Id() = %v, want %v", got, tt.want)
				}
			})
		}
	}
}

func TestAutoScalerServer_Debug(t *testing.T) {
	tests := []struct {
		name    string
		request *apigrpc.NodeGroupServiceRequest
		wantErr bool
	}{
		{
			name: "Debug",
			request: &apigrpc.NodeGroupServiceRequest{
				ProviderID:  testProviderID,
				NodeGroupID: testGroupID,
			},
		},
	}

	s, ctx, err := newTestServer(&testNodeGroup)

	if assert.NoError(t, err) {
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {

				_, err := s.Debug(ctx, tt.request)

				if (err != nil) != tt.wantErr {
					t.Errorf("AutoScalerServerApp.Debug() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	}
}

func extractInstanceID(instances *apigrpc.Instances) []string {
	r := make([]string, len(instances.GetItems()))

	for i, n := range instances.GetItems() {
		r[i] = n.GetId()
	}

	return r
}

func TestAutoScalerServer_Nodes(t *testing.T) {
	tests := []struct {
		name    string
		request *apigrpc.NodeGroupServiceRequest
		want    []string
		wantErr bool
	}{
		{
			name: "Nodes",
			want: []string{
				fmt.Sprintf("%s://%s/object?type=node&name=%s", testProviderID, testGroupID, testNodeName),
			},
			request: &apigrpc.NodeGroupServiceRequest{
				ProviderID:  testProviderID,
				NodeGroupID: testGroupID,
			},
		},
	}

	ng := AutoScalerServerNodeGroup{
		NodeGroupIdentifier: testGroupID,
		Machine: &MachineCharacteristic{
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

	s, ctx, err := newTestServer(&ng)

	if assert.NoError(t, err) {
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got, err := s.Nodes(ctx, tt.request)

				if (err != nil) != tt.wantErr {
					t.Errorf("AutoScalerServerApp.Nodes() error = %v, wantErr %v", err, tt.wantErr)
				} else if got.GetError() != nil {
					t.Errorf("AutoScalerServerApp.Nodes() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
				} else if !reflect.DeepEqual(extractInstanceID(got.GetInstances()), tt.want) {
					t.Errorf("AutoScalerServerApp.Nodes() = %v, want %v", extractInstanceID(got.GetInstances()), tt.want)
				}
			})
		}
	}
}

func TestAutoScalerServer_TemplateNodeInfo(t *testing.T) {
	type args struct {
		ctx context.Context
	}

	tests := []struct {
		name    string
		request *apigrpc.NodeGroupServiceRequest
		want    *apigrpc.TemplateNodeInfoReply
		wantErr bool
	}{
		{
			name: "TemplateNodeInfo",
			request: &apigrpc.NodeGroupServiceRequest{
				ProviderID:  testProviderID,
				NodeGroupID: testGroupID,
			},
		},
	}

	s, ctx, err := newTestServer(&testNodeGroup)

	if assert.NoError(t, err) {
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {

				got, err := s.TemplateNodeInfo(ctx, tt.request)

				if (err != nil) != tt.wantErr {
					t.Errorf("AutoScalerServerApp.TemplateNodeInfo() error = %v, wantErr %v", err, tt.wantErr)
				} else if got.GetError() != nil {
					t.Errorf("AutoScalerServerApp.TemplateNodeInfo() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
				}
			})
		}
	}
}

func TestAutoScalerServer_Exist(t *testing.T) {

	tests := []struct {
		name    string
		request *apigrpc.NodeGroupServiceRequest
		want    bool
		wantErr bool
	}{
		{
			name: "Exists",
			want: true,
			request: &apigrpc.NodeGroupServiceRequest{
				ProviderID:  testProviderID,
				NodeGroupID: testGroupID,
			},
		},
	}

	s, ctx, err := newTestServer(&testNodeGroup)

	if assert.NoError(t, err) {
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {

				got, err := s.Exist(ctx, tt.request)

				if (err != nil) != tt.wantErr {
					t.Errorf("AutoScalerServerApp.Exist() error = %v, wantErr %v", err, tt.wantErr)
				} else if got.GetExists() != tt.want {
					t.Errorf("AutoScalerServerApp.Exist() = %v, want %v", got.GetExists(), tt.want)
				}
			})
		}
	}
}

func TestAutoScalerServer_Create(t *testing.T) {

	tests := []struct {
		name    string
		request *apigrpc.NodeGroupServiceRequest
		want    string
		wantErr bool
	}{
		{
			name: "Create",
			want: testGroupID,
			request: &apigrpc.NodeGroupServiceRequest{
				ProviderID:  testProviderID,
				NodeGroupID: testGroupID,
			},
		},
	}

	s, ctx, err := newTestServer(&testNodeGroup)

	if assert.NoError(t, err) {
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {

				got, err := s.Create(ctx, tt.request)

				if (err != nil) != tt.wantErr {
					t.Errorf("AutoScalerServerApp.Create() error = %v, wantErr %v", err, tt.wantErr)
				} else if got.GetError() != nil {
					t.Errorf("AutoScalerServerApp.Create() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
				} else if got.GetNodeGroup().GetId() != tt.want {
					t.Errorf("AutoScalerServerApp.Create() = %v, want %v", got.GetNodeGroup().GetId(), tt.want)
				}
			})
		}
	}
}

func TestAutoScalerServer_Delete(t *testing.T) {
	tests := []struct {
		name    string
		request *apigrpc.NodeGroupServiceRequest
		want    *apigrpc.DeleteReply
		wantErr bool
	}{
		{
			name: "Delete",
			request: &apigrpc.NodeGroupServiceRequest{
				ProviderID:  testProviderID,
				NodeGroupID: testGroupID,
			},
		},
	}

	ng := AutoScalerServerNodeGroup{
		NodeGroupIdentifier: testGroupID,
		Machine: &MachineCharacteristic{
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

	s, ctx, err := newTestServer(&ng)

	if assert.NoError(t, err) {
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {

				got, err := s.Delete(ctx, tt.request)

				if (err != nil) != tt.wantErr {
					t.Errorf("AutoScalerServerApp.Delete() error = %v, wantErr %v", err, tt.wantErr)
				} else if got.GetError() != nil {
					t.Errorf("AutoScalerServerApp.Delete() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
				}
			})
		}
	}
}

func TestAutoScalerServer_Autoprovisioned(t *testing.T) {
	tests := []struct {
		name    string
		request *apigrpc.NodeGroupServiceRequest
		want    bool
		wantErr bool
	}{
		{
			name: "Autoprovisioned",
			want: true,
			request: &apigrpc.NodeGroupServiceRequest{
				ProviderID:  testProviderID,
				NodeGroupID: testGroupID,
			},
		},
	}

	s, ctx, err := newTestServer(&testNodeGroup)

	if assert.NoError(t, err) {
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {

				got, err := s.Autoprovisioned(ctx, tt.request)

				if (err != nil) != tt.wantErr {
					t.Errorf("AutoScalerServerApp.Autoprovisioned() error = %v, wantErr %v", err, tt.wantErr)
				} else if got.GetAutoprovisioned() != tt.want {
					t.Errorf("AutoScalerServerApp.Autoprovisioned() = %v, want %v", got.GetAutoprovisioned(), tt.want)
				}
			})
		}
	}
}

func TestAutoScalerServer_Belongs(t *testing.T) {
	tests := []struct {
		name    string
		request *apigrpc.BelongsRequest
		want    bool
		wantErr bool
	}{
		{
			name: "Belongs",
			want: true,
			request: &apigrpc.BelongsRequest{
				ProviderID:  testProviderID,
				NodeGroupID: testGroupID,
				Node: toJSON(
					apiv1.Node{
						Spec: apiv1.NodeSpec{
							ProviderID: fmt.Sprintf("%s://%s/object?type=node&name=%s", testProviderID, testGroupID, testNodeName),
						},
					},
				),
			},
		},
		{
			name:    "NotBelongs",
			want:    false,
			wantErr: false,
			request: &apigrpc.BelongsRequest{
				ProviderID:  testProviderID,
				NodeGroupID: testGroupID,
				Node: toJSON(
					apiv1.Node{
						Spec: apiv1.NodeSpec{
							ProviderID: fmt.Sprintf("%s://%s/object?type=node&name=wrong-name", testProviderID, testGroupID),
						},
					},
				),
			},
		},
	}

	ng := AutoScalerServerNodeGroup{
		NodeGroupIdentifier: testGroupID,
		Machine: &MachineCharacteristic{
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

	s, ctx, err := newTestServer(&ng)

	if assert.NoError(t, err) {
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {

				got, err := s.Belongs(ctx, tt.request)

				if (err != nil) != tt.wantErr {
					t.Errorf("AutoScalerServerApp.Belongs() error = %v, wantErr %v", err, tt.wantErr)
				} else if got.GetError() != nil {
					t.Errorf("AutoScalerServerApp.Belongs() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
				} else if got.GetBelongs() != tt.want {
					t.Errorf("AutoScalerServerApp.Belongs() = %v, want %v", got.GetBelongs(), tt.want)
				}
			})
		}
	}
}

func TestAutoScalerServer_NodePrice(t *testing.T) {
	tests := []struct {
		name    string
		request *apigrpc.NodePriceRequest
		want    float64
		wantErr bool
	}{
		{
			name: "NodePrice",
			want: 0,
			request: &apigrpc.NodePriceRequest{
				ProviderID: testProviderID,
				StartTime:  time.Now().Unix(),
				EndTime:    time.Now().Add(time.Hour).Unix(),
				Node: toJSON(apiv1.Node{
					Spec: apiv1.NodeSpec{
						ProviderID: fmt.Sprintf("%s://%s/object?type=node&name=%s", testProviderID, testGroupID, testNodeName),
					},
				},
				),
			},
		},
	}

	s, ctx, err := newTestServer(&testNodeGroup)

	if assert.NoError(t, err) {
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {

				got, err := s.NodePrice(ctx, tt.request)

				if (err != nil) != tt.wantErr {
					t.Errorf("AutoScalerServerApp.NodePrice() error = %v, wantErr %v", err, tt.wantErr)
				} else if got.GetError() != nil {
					t.Errorf("AutoScalerServerApp.NodePrice() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
				} else if got.GetPrice() != tt.want {
					t.Errorf("AutoScalerServerApp.NodePrice() = %v, want %v", got.GetPrice(), tt.want)
				}
			})
		}
	}
}

func TestAutoScalerServer_PodPrice(t *testing.T) {
	tests := []struct {
		name    string
		request *apigrpc.PodPriceRequest
		want    float64
		wantErr bool
	}{
		{
			name: "PodPrice",
			want: 0,
			request: &apigrpc.PodPriceRequest{
				ProviderID: testProviderID,
				StartTime:  time.Now().Unix(),
				EndTime:    time.Now().Add(time.Hour).Unix(),
				Pod: toJSON(
					apiv1.Pod{
						Spec: apiv1.PodSpec{
							NodeName: "test-instance-id",
						},
					},
				),
			},
		},
	}

	s, ctx, err := newTestServer(&testNodeGroup)

	if assert.NoError(t, err) {
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got, err := s.PodPrice(ctx, tt.request)
				if (err != nil) != tt.wantErr {
					t.Errorf("AutoScalerServerApp.PodPrice() error = %v, wantErr %v", err, tt.wantErr)
				} else if got.GetError() != nil {
					t.Errorf("AutoScalerServerApp.PodPrice() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
				} else if got.GetPrice() != tt.want {
					t.Errorf("AutoScalerServerApp.PodPrice() = %v, want %v", got.GetPrice(), tt.want)
				}
			})
		}
	}
}
