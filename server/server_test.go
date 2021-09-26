package server

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/Fred78290/kubernetes-vmware-autoscaler/constantes"
	apigrpc "github.com/Fred78290/kubernetes-vmware-autoscaler/grpc"
	"github.com/Fred78290/kubernetes-vmware-autoscaler/types"
	"github.com/Fred78290/kubernetes-vmware-autoscaler/utils"
	apiv1 "k8s.io/api/core/v1"
)

const (
	testProviderID = "vmware"
	testGroupID    = "afp-slyo-ca-k8s"
	testNodeName   = "afp-slyo-ca-k8s-vm-00"
)

func newTestServer(addNodeGroup, addTestNode bool) (*AutoScalerServerApp, *AutoScalerServerNodeGroup, context.Context, error) {

	config, ng, err := newTestNodeGroup()

	if err == nil {
		s := &AutoScalerServerApp{
			ResourceLimiter: &types.ResourceLimiter{
				MinLimits: map[string]int64{constantes.ResourceNameCores: 1, constantes.ResourceNameMemory: 10000000},
				MaxLimits: map[string]int64{constantes.ResourceNameCores: 5, constantes.ResourceNameMemory: 100000000},
			},
			Groups:        map[string]*AutoScalerServerNodeGroup{},
			Configuration: config,
		}

		if addNodeGroup {
			s.Groups[ng.NodeGroupIdentifier] = ng

			if addTestNode {
				node := createTestNode(ng)

				ng.Nodes[node.NodeName] = node
			}
		}

		return s, ng, context.TODO(), err
	}

	return nil, nil, nil, err
}

func extractNodeGroup(nodeGroups []*apigrpc.NodeGroup) []string {
	r := make([]string, len(nodeGroups))

	for i, n := range nodeGroups {
		r[i] = n.Id
	}

	return r
}

func TestAutoScalerServer_NodeGroups(t *testing.T) {
	s, _, ctx, err := newTestServer(true, false)

	expected := []string{
		testGroupID,
	}

	if assert.NoError(t, err) {
		t.Run("NodeGroups", func(t *testing.T) {
			request := &apigrpc.CloudProviderServiceRequest{
				ProviderID: testProviderID,
			}

			if got, err := s.NodeGroups(ctx, request); err != nil {
				t.Errorf("AutoScalerServerApp.NodeGroups() error = %v", err)
			} else if !reflect.DeepEqual(extractNodeGroup(got.GetNodeGroups()), expected) {
				t.Errorf("AutoScalerServerApp.NodeGroups() = %v, want %v", extractNodeGroup(got.GetNodeGroups()), expected)
			}
		})
	}
}

func TestAutoScalerServer_NodeGroupForNode(t *testing.T) {
	s, _, ctx, err := newTestServer(true, true)

	if assert.NoError(t, err) {
		t.Run("NodeGroupForNode", func(t *testing.T) {
			request := &apigrpc.NodeGroupForNodeRequest{
				ProviderID: testProviderID,
				Node: utils.ToJSON(
					apiv1.Node{
						Spec: apiv1.NodeSpec{
							ProviderID: fmt.Sprintf("%s://%s/object?type=node&name=%s", testProviderID, testGroupID, testNodeName),
						},
					},
				),
			}

			if got, err := s.NodeGroupForNode(ctx, request); err != nil {
				t.Errorf("AutoScalerServerApp.NodeGroupForNode() error = %v", err)
			} else if got.GetError() != nil {
				t.Errorf("AutoScalerServerApp.NodeGroupForNode() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
			} else if !reflect.DeepEqual(got.GetNodeGroup().GetId(), testGroupID) {
				t.Errorf("AutoScalerServerApp.NodeGroupForNode() = %v, want %v", got.GetNodeGroup().GetId(), testGroupID)
			}
		})
	}
}

func TestAutoScalerServer_Pricing(t *testing.T) {
	s, _, ctx, err := newTestServer(true, false)

	if assert.NoError(t, err) {
		t.Run("Pricing", func(t *testing.T) {
			request := &apigrpc.CloudProviderServiceRequest{
				ProviderID: testProviderID,
			}

			if got, err := s.Pricing(ctx, request); err != nil {
				t.Errorf("AutoScalerServerApp.Pricing() error = %v", err)
			} else if got.GetError() != nil {
				t.Errorf("AutoScalerServerApp.Pricing() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
			} else if !reflect.DeepEqual(got.GetPriceModel().GetId(), testProviderID) {
				t.Errorf("AutoScalerServerApp.Pricing() = %v, want %v", got.GetPriceModel().GetId(), testProviderID)
			}
		})
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
	s, _, ctx, err := newTestServer(true, false)

	expected := []string{
		"tiny",
		"medium",
		"large",
		"extra-large",
	}

	if assert.NoError(t, err) {
		t.Run("GetAvailableMachineTypes", func(t *testing.T) {
			request := &apigrpc.CloudProviderServiceRequest{
				ProviderID: testProviderID,
			}

			if got, err := s.GetAvailableMachineTypes(ctx, request); err != nil {
				t.Errorf("AutoScalerServerApp.GetAvailableMachineTypes() error = %v", err)
			} else if got.GetError() != nil {
				t.Errorf("AutoScalerServerApp.GetAvailableMachineTypes() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
			} else if !reflect.DeepEqual(extractAvailableMachineTypes(got.GetAvailableMachineTypes()), expected) {
				t.Errorf("AutoScalerServerApp.GetAvailableMachineTypes() = %v, want %v", extractAvailableMachineTypes(got.GetAvailableMachineTypes()), expected)
			}
		})
	}
}

func TestAutoScalerServer_NewNodeGroup(t *testing.T) {
	s, _, ctx, err := newTestServer(false, false)

	if assert.NoError(t, err) {
		t.Run("NewNodeGroup", func(t *testing.T) {

			request := &apigrpc.NewNodeGroupRequest{
				ProviderID:  testProviderID,
				MachineType: "tiny",
				Labels: KubernetesLabel{
					"database": "true",
					"cluster":  "true",
				},
			}
			if got, err := s.NewNodeGroup(ctx, request); err != nil {
				t.Errorf("AutoScalerServerApp.NewNodeGroup() error = %v", err)
			} else if got.GetError() != nil {
				t.Errorf("AutoScalerServerApp.NewNodeGroup() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
			} else {
				t.Logf("AutoScalerServerApp.NewNodeGroup() return node group created :%v", got.GetNodeGroup().GetId())
			}
		})
	}
}

func extractResourceLimiter(res *apigrpc.ResourceLimiter) *types.ResourceLimiter {
	r := &types.ResourceLimiter{
		MinLimits: res.MinLimits,
		MaxLimits: res.MaxLimits,
	}

	return r
}

func TestAutoScalerServer_GetResourceLimiter(t *testing.T) {
	s, _, ctx, err := newTestServer(true, false)

	expected := &types.ResourceLimiter{
		MinLimits: map[string]int64{constantes.ResourceNameCores: 1, constantes.ResourceNameMemory: 10000000},
		MaxLimits: map[string]int64{constantes.ResourceNameCores: 5, constantes.ResourceNameMemory: 100000000},
	}

	if assert.NoError(t, err) {
		t.Run("GetResourceLimiter", func(t *testing.T) {
			request := &apigrpc.CloudProviderServiceRequest{
				ProviderID: testProviderID,
			}

			if got, err := s.GetResourceLimiter(ctx, request); err != nil {
				t.Errorf("AutoScalerServerApp.GetResourceLimiter() error = %v", err)
			} else if got.GetError() != nil {
				t.Errorf("AutoScalerServerApp.GetResourceLimiter() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
			} else if !reflect.DeepEqual(extractResourceLimiter(got.GetResourceLimiter()), expected) {
				t.Errorf("AutoScalerServerApp.GetResourceLimiter() = %v, want %v", got.GetResourceLimiter(), expected)
			}
		})
	}
}

func TestAutoScalerServer_Cleanup(t *testing.T) {
	s, _, ctx, err := newTestServer(true, true)

	if assert.NoError(t, err) {
		t.Run("Cleanup", func(t *testing.T) {
			request := &apigrpc.CloudProviderServiceRequest{
				ProviderID: testProviderID,
			}

			if got, err := s.Cleanup(ctx, request); err != nil {
				t.Errorf("AutoScalerServerApp.Cleanup() error = %v", err)
			} else if got.GetError() != nil {
				t.Errorf("AutoScalerServerApp.Cleanup() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
			}
		})
	}
}

func TestAutoScalerServer_Refresh(t *testing.T) {
	s, _, ctx, err := newTestServer(true, true)

	if assert.NoError(t, err) {
		t.Run("Refresh", func(t *testing.T) {
			request := &apigrpc.CloudProviderServiceRequest{
				ProviderID: testProviderID,
			}

			if got, err := s.Refresh(ctx, request); err != nil {
				t.Errorf("AutoScalerServerApp.Refresh() error = %v", err)
			} else if got.GetError() != nil {
				t.Errorf("AutoScalerServerApp.Refresh() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
			}
		})
	}
}

func TestAutoScalerServer_MaxSize(t *testing.T) {
	s, ng, ctx, err := newTestServer(true, false)

	if assert.NoError(t, err) {
		t.Run("MaxSize", func(t *testing.T) {
			request := &apigrpc.NodeGroupServiceRequest{
				ProviderID:  testProviderID,
				NodeGroupID: testGroupID,
			}

			if got, err := s.MaxSize(ctx, request); err != nil {
				t.Errorf("AutoScalerServerApp.MaxSize() error = %v", err)
			} else if got.GetMaxSize() != int32(ng.MaxNodeSize) {
				t.Errorf("AutoScalerServerApp.MaxSize() = %v, want %v", got.GetMaxSize(), ng.MaxNodeSize)
			}
		})
	}
}

func TestAutoScalerServer_MinSize(t *testing.T) {
	s, ng, ctx, err := newTestServer(true, false)

	if assert.NoError(t, err) {
		t.Run("MinSize", func(t *testing.T) {
			request := &apigrpc.NodeGroupServiceRequest{
				ProviderID:  testProviderID,
				NodeGroupID: testGroupID,
			}

			if got, err := s.MinSize(ctx, request); err != nil {
				t.Errorf("AutoScalerServerApp.MinSize() error = %v", err)
			} else if got.GetMinSize() != int32(ng.MinNodeSize) {
				t.Errorf("AutoScalerServerApp.MinSize() = %v, want %v", got.GetMinSize(), ng.MinNodeSize)
			}
		})
	}
}

func TestAutoScalerServer_TargetSize(t *testing.T) {
	s, _, ctx, err := newTestServer(true, true)

	if assert.NoError(t, err) {
		t.Run("TargetSize", func(t *testing.T) {
			request := &apigrpc.NodeGroupServiceRequest{
				ProviderID:  testProviderID,
				NodeGroupID: testGroupID,
			}

			if got, err := s.TargetSize(ctx, request); err != nil {
				t.Errorf("AutoScalerServerApp.TargetSize() error = %v", err)
			} else if got.GetError() != nil {
				t.Errorf("AutoScalerServerApp.TargetSize() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
			} else if got.GetTargetSize() != 1 {
				t.Errorf("AutoScalerServerApp.TargetSize() = %v, want %v", got.GetTargetSize(), 1)
			}
		})
	}
}

func TestAutoScalerServer_IncreaseSize(t *testing.T) {
	s, _, ctx, err := newTestServer(true, false)

	if assert.NoError(t, err) {
		t.Run("IncreaseSize", func(t *testing.T) {
			request := &apigrpc.IncreaseSizeRequest{
				ProviderID:  testProviderID,
				NodeGroupID: testGroupID,
				Delta:       1,
			}

			if got, err := s.IncreaseSize(ctx, request); err != nil {
				t.Errorf("AutoScalerServerApp.IncreaseSize() error = %v", err)
			} else if got.GetError() != nil {
				t.Errorf("AutoScalerServerApp.IncreaseSize() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
			}
		})
	}
}

func TestAutoScalerServer_DeleteNodes(t *testing.T) {
	s, _, ctx, err := newTestServer(true, true)

	if assert.NoError(t, err) {
		t.Run("DeleteNodes", func(t *testing.T) {
			request := &apigrpc.DeleteNodesRequest{
				ProviderID:  testProviderID,
				NodeGroupID: testGroupID,
				Node: []string{
					utils.ToJSON(
						apiv1.Node{
							Spec: apiv1.NodeSpec{
								ProviderID: fmt.Sprintf("%s://%s/object?type=node&name=%s", testProviderID, testGroupID, testNodeName),
							},
						},
					),
				},
			}

			if got, err := s.DeleteNodes(ctx, request); err != nil {
				t.Errorf("AutoScalerServerApp.DeleteNodes() error = %v", err)
			} else if got.GetError() != nil {
				t.Errorf("AutoScalerServerApp.DeleteNodes() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
			}
		})
	}
}

func TestAutoScalerServer_DecreaseTargetSize(t *testing.T) {
	s, _, ctx, err := newTestServer(true, true)

	if assert.NoError(t, err) {
		t.Run("DecreaseTargetSize", func(t *testing.T) {
			request := &apigrpc.DecreaseTargetSizeRequest{
				ProviderID:  testProviderID,
				NodeGroupID: testGroupID,
				Delta:       -1,
			}

			if got, err := s.DecreaseTargetSize(ctx, request); err != nil {
				t.Errorf("AutoScalerServerApp.DecreaseTargetSize() error = %v", err)
			} else if got.GetError() != nil {
				t.Errorf("AutoScalerServerApp.DecreaseTargetSize() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
			}
		})
	}
}

func TestAutoScalerServer_Id(t *testing.T) {
	s, _, ctx, err := newTestServer(true, false)

	if assert.NoError(t, err) {
		t.Run("Id", func(t *testing.T) {
			request := &apigrpc.NodeGroupServiceRequest{
				ProviderID:  testProviderID,
				NodeGroupID: testGroupID,
			}

			if got, err := s.Id(ctx, request); err != nil {
				t.Errorf("AutoScalerServerApp.Id() error = %v", err)
			} else if got.GetResponse() != testGroupID {
				t.Errorf("AutoScalerServerApp.Id() = %v, want %v", got, testGroupID)
			}
		})
	}
}

func TestAutoScalerServer_Debug(t *testing.T) {
	s, _, ctx, err := newTestServer(true, false)

	if assert.NoError(t, err) {
		t.Run("Debug", func(t *testing.T) {
			request := &apigrpc.NodeGroupServiceRequest{
				ProviderID:  testProviderID,
				NodeGroupID: testGroupID,
			}

			if _, err := s.Debug(ctx, request); err != nil {
				t.Errorf("AutoScalerServerApp.Debug() error = %v", err)
			}
		})
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
	s, _, ctx, err := newTestServer(true, true)

	expected := []string{
		fmt.Sprintf("%s://%s/object?type=node&name=%s", testProviderID, testGroupID, testNodeName),
	}

	if assert.NoError(t, err) {
		t.Run("Nodes", func(t *testing.T) {
			request := &apigrpc.NodeGroupServiceRequest{
				ProviderID:  testProviderID,
				NodeGroupID: testGroupID,
			}

			if got, err := s.Nodes(ctx, request); err != nil {
				t.Errorf("AutoScalerServerApp.Nodes() error = %v", err)
			} else if got.GetError() != nil {
				t.Errorf("AutoScalerServerApp.Nodes() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
			} else if !reflect.DeepEqual(extractInstanceID(got.GetInstances()), expected) {
				t.Errorf("AutoScalerServerApp.Nodes() = %v, want %v", extractInstanceID(got.GetInstances()), expected)
			}
		})
	}
}

func TestAutoScalerServer_TemplateNodeInfo(t *testing.T) {
	s, _, ctx, err := newTestServer(true, true)

	if assert.NoError(t, err) {
		t.Run("TemplateNodeInfo", func(t *testing.T) {
			request := &apigrpc.NodeGroupServiceRequest{
				ProviderID:  testProviderID,
				NodeGroupID: testGroupID,
			}

			if got, err := s.TemplateNodeInfo(ctx, request); err != nil {
				t.Errorf("AutoScalerServerApp.TemplateNodeInfo() error = %v", err)
			} else if got.GetError() != nil {
				t.Errorf("AutoScalerServerApp.TemplateNodeInfo() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
			}
		})
	}
}

func TestAutoScalerServer_Exist(t *testing.T) {
	s, _, ctx, err := newTestServer(true, false)

	if assert.NoError(t, err) {
		t.Run("Exists", func(t *testing.T) {
			request := &apigrpc.NodeGroupServiceRequest{
				ProviderID:  testProviderID,
				NodeGroupID: testGroupID,
			}

			if got, err := s.Exist(ctx, request); err != nil {
				t.Errorf("AutoScalerServerApp.Exist() error = %v", err)
			} else if got.GetExists() == false {
				t.Errorf("AutoScalerServerApp.Exist() = %v", got.GetExists())
			}
		})
	}
}

func TestAutoScalerServer_Create(t *testing.T) {
	s, _, ctx, err := newTestServer(true, false)

	if assert.NoError(t, err) {
		t.Run("Create", func(t *testing.T) {
			request := &apigrpc.NodeGroupServiceRequest{
				ProviderID:  testProviderID,
				NodeGroupID: testGroupID,
			}

			if got, err := s.Create(ctx, request); err != nil {
				t.Errorf("AutoScalerServerApp.Create() error = %v", err)
			} else if got.GetError() != nil {
				t.Errorf("AutoScalerServerApp.Create() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
			} else if got.GetNodeGroup().GetId() != testGroupID {
				t.Errorf("AutoScalerServerApp.Create() = %v, want %v", got.GetNodeGroup().GetId(), testGroupID)
			}
		})
	}
}

func TestAutoScalerServer_Delete(t *testing.T) {
	s, _, ctx, err := newTestServer(true, false)

	if assert.NoError(t, err) {
		t.Run("Delete", func(t *testing.T) {
			request := &apigrpc.NodeGroupServiceRequest{
				ProviderID:  testProviderID,
				NodeGroupID: testGroupID,
			}

			if got, err := s.Delete(ctx, request); err != nil {
				t.Errorf("AutoScalerServerApp.Delete() error = %v", err)
			} else if got.GetError() != nil {
				t.Errorf("AutoScalerServerApp.Delete() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
			}
		})
	}
}

func TestAutoScalerServer_Autoprovisioned(t *testing.T) {
	s, _, ctx, err := newTestServer(true, false)

	if assert.NoError(t, err) {
		t.Run("Autoprovisioned", func(t *testing.T) {
			request := &apigrpc.NodeGroupServiceRequest{
				ProviderID:  testProviderID,
				NodeGroupID: testGroupID,
			}

			if got, err := s.Autoprovisioned(ctx, request); err != nil {
				t.Errorf("AutoScalerServerApp.Autoprovisioned() error = %v", err)
			} else if got.GetAutoprovisioned() == false {
				t.Errorf("AutoScalerServerApp.Autoprovisioned() = %v, want true", got.GetAutoprovisioned())
			}
		})
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
				Node: utils.ToJSON(
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
				Node: utils.ToJSON(
					apiv1.Node{
						Spec: apiv1.NodeSpec{
							ProviderID: fmt.Sprintf("%s://%s/object?type=node&name=%s", testProviderID, testGroupID, "wrong-name"),
						},
					},
				),
			},
		},
	}

	s, _, ctx, err := newTestServer(true, true)

	if assert.NoError(t, err) {
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {

				got, err := s.Belongs(ctx, test.request)

				if (err != nil) != test.wantErr {
					t.Errorf("AutoScalerServerApp.Belongs() error = %v", err)
				} else if got.GetError() != nil {
					t.Errorf("AutoScalerServerApp.Belongs() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
				} else if got.GetBelongs() != test.want {
					t.Errorf("AutoScalerServerApp.Belongs() = %v, want %v", got.GetBelongs(), test.want)
				}
			})
		}
	}
}

func TestAutoScalerServer_NodePrice(t *testing.T) {
	s, _, ctx, err := newTestServer(true, true)

	if assert.NoError(t, err) {
		t.Run("Node Price", func(t *testing.T) {

			request := &apigrpc.NodePriceRequest{
				ProviderID: testProviderID,
				StartTime:  time.Now().Unix(),
				EndTime:    time.Now().Add(time.Hour).Unix(),
				Node: utils.ToJSON(apiv1.Node{
					Spec: apiv1.NodeSpec{
						ProviderID: fmt.Sprintf("%s://%s/object?type=node&name=%s", testProviderID, testGroupID, testNodeName),
					},
				},
				),
			}

			if got, err := s.NodePrice(ctx, request); err != nil {
				t.Errorf("AutoScalerServerApp.NodePrice() error = %v", err)
			} else if got.GetError() != nil {
				t.Errorf("AutoScalerServerApp.NodePrice() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
			} else if got.GetPrice() != 0 {
				t.Errorf("AutoScalerServerApp.NodePrice() = %v, want %v", got.GetPrice(), 0)
			}
		})
	}
}

func TestAutoScalerServer_PodPrice(t *testing.T) {
	s, _, ctx, err := newTestServer(true, true)

	if assert.NoError(t, err) {
		t.Run("Pod Price", func(t *testing.T) {
			request := &apigrpc.PodPriceRequest{
				ProviderID: testProviderID,
				StartTime:  time.Now().Unix(),
				EndTime:    time.Now().Add(time.Hour).Unix(),
				Pod: utils.ToJSON(apiv1.Pod{
					Spec: apiv1.PodSpec{
						NodeName: testNodeName,
					},
				},
				),
			}

			if got, err := s.PodPrice(ctx, request); err != nil {
				t.Errorf("AutoScalerServerApp.PodPrice() error = %v", err)
			} else if got.GetError() != nil {
				t.Errorf("AutoScalerServerApp.PodPrice() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
			} else if got.GetPrice() != 0 {
				t.Errorf("AutoScalerServerApp.PodPrice() = %v, want %v", got.GetPrice(), 0)
			}
		})
	}
}
