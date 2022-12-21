package server

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/Fred78290/kubernetes-vmware-autoscaler/constantes"
	apigrpc "github.com/Fred78290/kubernetes-vmware-autoscaler/grpc"
	"github.com/Fred78290/kubernetes-vmware-autoscaler/types"
	"github.com/Fred78290/kubernetes-vmware-autoscaler/utils"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	testServiceIdentifier = "vmware"
	testGroupID           = "vmware-ca-k8s"
	testNodeName          = "DC0_H0_VM0"
	testVMUUID            = "265104de-1472-547c-b873-6dc7883fb6cb"
	testCRDUID            = "96cb1c71-1d2e-4c55-809f-72e874fc4b2c"
	launchVMName          = "vmware-ca-k8s-autoscaled-01"
)

type serverTest struct {
	t *testing.T
}

func (m *serverTest) NodeGroups() {
	s, _, ctx, err := newTestServer(true, false)

	expected := []string{
		testGroupID,
	}

	if assert.NoError(m.t, err) {
		request := &apigrpc.CloudProviderServiceRequest{
			ProviderID: testServiceIdentifier,
		}

		if got, err := s.NodeGroups(ctx, request); err != nil {
			m.t.Errorf("AutoScalerServerApp.NodeGroups() error = %v", err)
		} else if !reflect.DeepEqual(extractNodeGroup(got.GetNodeGroups()), expected) {
			m.t.Errorf("AutoScalerServerApp.NodeGroups() = %v, want %v", extractNodeGroup(got.GetNodeGroups()), expected)
		}
	}

}

func (m *serverTest) NodeGroupForNode() {
	s, _, ctx, err := newTestServer(true, true)

	if assert.NoError(m.t, err) {
		request := &apigrpc.NodeGroupForNodeRequest{
			ProviderID: testServiceIdentifier,
			Node: utils.ToJSON(
				apiv1.Node{
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
				},
			),
		}

		if got, err := s.NodeGroupForNode(ctx, request); err != nil {
			m.t.Errorf("AutoScalerServerApp.NodeGroupForNode() error = %v", err)
		} else if got.GetError() != nil {
			m.t.Errorf("AutoScalerServerApp.NodeGroupForNode() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
		} else if !reflect.DeepEqual(got.GetNodeGroup().GetId(), testGroupID) {
			m.t.Errorf("AutoScalerServerApp.NodeGroupForNode() = %v, want %v", got.GetNodeGroup().GetId(), testGroupID)
		}
	}
}

func (m *serverTest) Pricing() {
	s, _, ctx, err := newTestServer(true, false)

	if assert.NoError(m.t, err) {
		request := &apigrpc.CloudProviderServiceRequest{
			ProviderID: testServiceIdentifier,
		}

		if got, err := s.Pricing(ctx, request); err != nil {
			m.t.Errorf("AutoScalerServerApp.Pricing() error = %v", err)
		} else if got.GetError() != nil {
			m.t.Errorf("AutoScalerServerApp.Pricing() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
		} else if !reflect.DeepEqual(got.GetPriceModel().GetId(), testServiceIdentifier) {
			m.t.Errorf("AutoScalerServerApp.Pricing() = %v, want %v", got.GetPriceModel().GetId(), testServiceIdentifier)
		}
	}
}

func (m *serverTest) GetAvailableMachineTypes() {
	s, _, ctx, err := newTestServer(true, false)

	expected := []string{
		"2xlarge",
		"4xlarge",
		"large",
		"medium",
		"small",
		"tiny",
		"xlarge",
	}

	sort.Strings(expected)

	if assert.NoError(m.t, err) {
		request := &apigrpc.CloudProviderServiceRequest{
			ProviderID: testServiceIdentifier,
		}

		if got, err := s.GetAvailableMachineTypes(ctx, request); err != nil {
			m.t.Errorf("AutoScalerServerApp.GetAvailableMachineTypes() error = %v", err)
		} else if got.GetError() != nil {
			m.t.Errorf("AutoScalerServerApp.GetAvailableMachineTypes() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
		} else if !reflect.DeepEqual(extractAvailableMachineTypes(got.GetAvailableMachineTypes()), expected) {
			m.t.Errorf("AutoScalerServerApp.GetAvailableMachineTypes() = %v, want %v", extractAvailableMachineTypes(got.GetAvailableMachineTypes()), expected)
		}
	}
}

func (m *serverTest) NewNodeGroup() {
	s, _, ctx, err := newTestServer(false, false)

	if assert.NoError(m.t, err) {
		request := &apigrpc.NewNodeGroupRequest{
			ProviderID:  testServiceIdentifier,
			MachineType: "tiny",
			Labels: types.KubernetesLabel{
				"database": "true",
				"cluster":  "true",
			},
		}

		if got, err := s.NewNodeGroup(ctx, request); err != nil {
			m.t.Errorf("AutoScalerServerApp.NewNodeGroup() error = %v", err)
		} else if got.GetError() != nil {
			m.t.Errorf("AutoScalerServerApp.NewNodeGroup() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
		} else {
			m.t.Logf("AutoScalerServerApp.NewNodeGroup() return node group created :%v", got.GetNodeGroup().GetId())
		}
	}
}

func (m *serverTest) GetResourceLimiter() {
	s, _, ctx, err := newTestServer(true, false)

	expected := &types.ResourceLimiter{
		MinLimits: map[string]int64{constantes.ResourceNameCores: 1, constantes.ResourceNameMemory: 10000000},
		MaxLimits: map[string]int64{constantes.ResourceNameCores: 5, constantes.ResourceNameMemory: 100000000},
	}

	if assert.NoError(m.t, err) {
		request := &apigrpc.CloudProviderServiceRequest{
			ProviderID: testServiceIdentifier,
		}

		if got, err := s.GetResourceLimiter(ctx, request); err != nil {
			m.t.Errorf("AutoScalerServerApp.GetResourceLimiter() error = %v", err)
		} else if got.GetError() != nil {
			m.t.Errorf("AutoScalerServerApp.GetResourceLimiter() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
		} else if !reflect.DeepEqual(extractResourceLimiter(got.GetResourceLimiter()), expected) {
			m.t.Errorf("AutoScalerServerApp.GetResourceLimiter() = %v, want %v", got.GetResourceLimiter(), expected)
		}
	}
}

func (m *serverTest) Cleanup() {
	s, _, ctx, err := newTestServer(true, true)

	if assert.NoError(m.t, err) {
		request := &apigrpc.CloudProviderServiceRequest{
			ProviderID: testServiceIdentifier,
		}

		if got, err := s.Cleanup(ctx, request); err != nil {
			m.t.Errorf("AutoScalerServerApp.Cleanup() error = %v", err)
		} else if got.GetError() != nil && got.GetError().GetReason() != "vm 'DC0_H0_VM0' not found" {
			m.t.Errorf("AutoScalerServerApp.Cleanup() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
		}
	}
}

func (m *serverTest) Refresh() {
	s, _, ctx, err := newTestServer(true, true)

	if assert.NoError(m.t, err) {
		request := &apigrpc.CloudProviderServiceRequest{
			ProviderID: testServiceIdentifier,
		}

		if got, err := s.Refresh(ctx, request); err != nil {
			m.t.Errorf("AutoScalerServerApp.Refresh() error = %v", err)
		} else if got.GetError() != nil {
			m.t.Errorf("AutoScalerServerApp.Refresh() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
		}
	}
}

func (m *serverTest) MaxSize() {
	s, ng, ctx, err := newTestServer(true, false)

	if assert.NoError(m.t, err) {
		request := &apigrpc.NodeGroupServiceRequest{
			ProviderID:  testServiceIdentifier,
			NodeGroupID: testGroupID,
		}

		if got, err := s.MaxSize(ctx, request); err != nil {
			m.t.Errorf("AutoScalerServerApp.MaxSize() error = %v", err)
		} else if got.GetMaxSize() != int32(ng.MaxNodeSize) {
			m.t.Errorf("AutoScalerServerApp.MaxSize() = %v, want %v", got.GetMaxSize(), ng.MaxNodeSize)
		}
	}
}

func (m *serverTest) MinSize() {
	s, ng, ctx, err := newTestServer(true, false)

	if assert.NoError(m.t, err) {
		request := &apigrpc.NodeGroupServiceRequest{
			ProviderID:  testServiceIdentifier,
			NodeGroupID: testGroupID,
		}

		if got, err := s.MinSize(ctx, request); err != nil {
			m.t.Errorf("AutoScalerServerApp.MinSize() error = %v", err)
		} else if got.GetMinSize() != int32(ng.MinNodeSize) {
			m.t.Errorf("AutoScalerServerApp.MinSize() = %v, want %v", got.GetMinSize(), ng.MinNodeSize)
		}
	}
}

func (m *serverTest) TargetSize() {
	s, _, ctx, err := newTestServer(true, true)

	if assert.NoError(m.t, err) {
		request := &apigrpc.NodeGroupServiceRequest{
			ProviderID:  testServiceIdentifier,
			NodeGroupID: testGroupID,
		}

		if got, err := s.TargetSize(ctx, request); err != nil {
			m.t.Errorf("AutoScalerServerApp.TargetSize() error = %v", err)
		} else if got.GetError() != nil {
			m.t.Errorf("AutoScalerServerApp.TargetSize() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
		} else if got.GetTargetSize() != 1 {
			m.t.Errorf("AutoScalerServerApp.TargetSize() = %v, want %v", got.GetTargetSize(), 1)
		}
	}
}

func (m *serverTest) IncreaseSize() {
	s, _, ctx, err := newTestServer(true, false)

	if assert.NoError(m.t, err) {
		request := &apigrpc.IncreaseSizeRequest{
			ProviderID:  testServiceIdentifier,
			NodeGroupID: testGroupID,
			Delta:       1,
		}

		if got, err := s.IncreaseSize(ctx, request); err != nil {
			m.t.Errorf("AutoScalerServerApp.IncreaseSize() error = %v", err)
		} else if got.GetError() != nil {
			m.t.Errorf("AutoScalerServerApp.IncreaseSize() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
		}
	}

}

func (m *serverTest) DecreaseTargetSize() {
	s, _, ctx, err := newTestServer(true, true)

	if assert.NoError(m.t, err) {
		request := &apigrpc.DecreaseTargetSizeRequest{
			ProviderID:  testServiceIdentifier,
			NodeGroupID: testGroupID,
			Delta:       -1,
		}

		if got, err := s.DecreaseTargetSize(ctx, request); err != nil {
			m.t.Errorf("AutoScalerServerApp.DecreaseTargetSize() error = %v", err)
		} else if got.GetError() != nil {
			if strings.HasPrefix(got.GetError().Reason, "attempt to delete existing nodes, targetSize") == false {
				m.t.Errorf("AutoScalerServerApp.DecreaseTargetSize() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
			}
		}
	}
}

func (m *serverTest) DeleteNodes() {
	s, _, ctx, err := newTestServer(true, true)

	if assert.NoError(m.t, err) {
		request := &apigrpc.DeleteNodesRequest{
			ProviderID:  testServiceIdentifier,
			NodeGroupID: testGroupID,
			Node: []string{
				utils.ToJSON(
					apiv1.Node{
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
					},
				),
			},
		}

		if got, err := s.DeleteNodes(ctx, request); err != nil {
			m.t.Errorf("AutoScalerServerApp.DeleteNodes() error = %v", err)
		} else if got.GetError() != nil {
			m.t.Errorf("AutoScalerServerApp.DeleteNodes() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
		}
	}
}

func (m *serverTest) Id() {
	s, _, ctx, err := newTestServer(true, false)

	if assert.NoError(m.t, err) {
		request := &apigrpc.NodeGroupServiceRequest{
			ProviderID:  testServiceIdentifier,
			NodeGroupID: testGroupID,
		}

		if got, err := s.Id(ctx, request); err != nil {
			m.t.Errorf("AutoScalerServerApp.Id() error = %v", err)
		} else if got.GetResponse() != testGroupID {
			m.t.Errorf("AutoScalerServerApp.Id() = %v, want %v", got, testGroupID)
		}
	}
}

func (m *serverTest) Debug() {
	s, _, ctx, err := newTestServer(true, false)

	if assert.NoError(m.t, err) {
		request := &apigrpc.NodeGroupServiceRequest{
			ProviderID:  testServiceIdentifier,
			NodeGroupID: testGroupID,
		}

		if _, err := s.Debug(ctx, request); err != nil {
			m.t.Errorf("AutoScalerServerApp.Debug() error = %v", err)
		}
	}
}

func (m *serverTest) Nodes() {
	s, _, ctx, err := newTestServer(true, true)

	expected := []string{
		fmt.Sprintf("vsphere://%s", testVMUUID),
	}

	if assert.NoError(m.t, err) {
		request := &apigrpc.NodeGroupServiceRequest{
			ProviderID:  testServiceIdentifier,
			NodeGroupID: testGroupID,
		}

		if got, err := s.Nodes(ctx, request); err != nil {
			m.t.Errorf("AutoScalerServerApp.Nodes() error = %v", err)
		} else if got.GetError() != nil {
			m.t.Errorf("AutoScalerServerApp.Nodes() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
		} else if !reflect.DeepEqual(extractInstanceID(got.GetInstances()), expected) {
			m.t.Errorf("AutoScalerServerApp.Nodes() = %v, want %v", extractInstanceID(got.GetInstances()), expected)
		}
	}
}

func (m *serverTest) TemplateNodeInfo() {
	s, _, ctx, err := newTestServer(true, true)

	if assert.NoError(m.t, err) {
		request := &apigrpc.NodeGroupServiceRequest{
			ProviderID:  testServiceIdentifier,
			NodeGroupID: testGroupID,
		}

		if got, err := s.TemplateNodeInfo(ctx, request); err != nil {
			m.t.Errorf("AutoScalerServerApp.TemplateNodeInfo() error = %v", err)
		} else if got.GetError() != nil {
			m.t.Errorf("AutoScalerServerApp.TemplateNodeInfo() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
		}
	}
}

func (m *serverTest) Exist() {
	s, _, ctx, err := newTestServer(true, false)

	if assert.NoError(m.t, err) {
		request := &apigrpc.NodeGroupServiceRequest{
			ProviderID:  testServiceIdentifier,
			NodeGroupID: testGroupID,
		}

		if got, err := s.Exist(ctx, request); err != nil {
			m.t.Errorf("AutoScalerServerApp.Exist() error = %v", err)
		} else if got.GetExists() == false {
			m.t.Errorf("AutoScalerServerApp.Exist() = %v", got.GetExists())
		}
	}
}

func (m *serverTest) Create() {
	s, _, ctx, err := newTestServer(true, false)

	if assert.NoError(m.t, err) {
		request := &apigrpc.NodeGroupServiceRequest{
			ProviderID:  testServiceIdentifier,
			NodeGroupID: testGroupID,
		}

		if got, err := s.Create(ctx, request); err != nil {
			m.t.Errorf("AutoScalerServerApp.Create() error = %v", err)
		} else if got.GetError() != nil {
			m.t.Errorf("AutoScalerServerApp.Create() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
		} else if got.GetNodeGroup().GetId() != testGroupID {
			m.t.Errorf("AutoScalerServerApp.Create() = %v, want %v", got.GetNodeGroup().GetId(), testGroupID)
		}
	}
}

func (m *serverTest) Delete() {
	s, _, ctx, err := newTestServer(true, false)

	if assert.NoError(m.t, err) {
		request := &apigrpc.NodeGroupServiceRequest{
			ProviderID:  testServiceIdentifier,
			NodeGroupID: testGroupID,
		}

		if got, err := s.Delete(ctx, request); err != nil {
			m.t.Errorf("AutoScalerServerApp.Delete() error = %v", err)
		} else if got.GetError() != nil {
			m.t.Errorf("AutoScalerServerApp.Delete() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
		}
	}
}

func (m *serverTest) Autoprovisioned() {
	s, _, ctx, err := newTestServer(true, false)

	if assert.NoError(m.t, err) {
		request := &apigrpc.NodeGroupServiceRequest{
			ProviderID:  testServiceIdentifier,
			NodeGroupID: testGroupID,
		}

		if got, err := s.Autoprovisioned(ctx, request); err != nil {
			m.t.Errorf("AutoScalerServerApp.Autoprovisioned() error = %v", err)
		} else if got.GetAutoprovisioned() == false {
			m.t.Errorf("AutoScalerServerApp.Autoprovisioned() = %v, want true", got.GetAutoprovisioned())
		}
	}
}

func (m *serverTest) Belongs() {
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
				ProviderID:  testServiceIdentifier,
				NodeGroupID: testGroupID,
				Node: utils.ToJSON(
					apiv1.Node{
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
					},
				),
			},
		},
		{
			name:    "NotBelongs",
			want:    false,
			wantErr: false,
			request: &apigrpc.BelongsRequest{
				ProviderID:  testServiceIdentifier,
				NodeGroupID: testGroupID,
				Node: utils.ToJSON(
					apiv1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name: "wrong-name",
							UID:  testCRDUID,
							Annotations: map[string]string{
								constantes.AnnotationNodeGroupName:        testGroupID,
								constantes.AnnotationNodeIndex:            "0",
								constantes.AnnotationInstanceID:           testVMUUID,
								constantes.AnnotationNodeAutoProvisionned: "true",
							},
						},
					},
				),
			},
		},
	}

	s, _, ctx, err := newTestServer(true, true)

	if assert.NoError(m.t, err) {
		for _, test := range tests {
			got, err := s.Belongs(ctx, test.request)

			if (err != nil) != test.wantErr {
				m.t.Errorf("AutoScalerServerApp.Belongs() error = %v", err)
			} else if got.GetError() != nil {
				m.t.Errorf("AutoScalerServerApp.Belongs() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
			} else if got.GetBelongs() != test.want {
				m.t.Errorf("AutoScalerServerApp.Belongs() = %v, want %v", got.GetBelongs(), test.want)
			}
		}
	}
}

func (m *serverTest) NodePrice() {
	s, _, ctx, err := newTestServer(true, true)

	if assert.NoError(m.t, err) {
		request := &apigrpc.NodePriceRequest{
			ProviderID: testServiceIdentifier,
			StartTime:  time.Now().Unix(),
			EndTime:    time.Now().Add(time.Hour).Unix(),
			Node: utils.ToJSON(
				apiv1.Node{
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
				},
			),
		}

		if got, err := s.NodePrice(ctx, request); err != nil {
			m.t.Errorf("AutoScalerServerApp.NodePrice() error = %v", err)
		} else if got.GetError() != nil {
			m.t.Errorf("AutoScalerServerApp.NodePrice() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
		} else if got.GetPrice() != 0 {
			m.t.Errorf("AutoScalerServerApp.NodePrice() = %v, want %v", got.GetPrice(), 0)
		}
	}
}

func (m *serverTest) PodPrice() {
	s, _, ctx, err := newTestServer(true, true)

	if assert.NoError(m.t, err) {
		request := &apigrpc.PodPriceRequest{
			ProviderID: testServiceIdentifier,
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
			m.t.Errorf("AutoScalerServerApp.PodPrice() error = %v", err)
		} else if got.GetError() != nil {
			m.t.Errorf("AutoScalerServerApp.PodPrice() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
		} else if got.GetPrice() != 0 {
			m.t.Errorf("AutoScalerServerApp.PodPrice() = %v, want %v", got.GetPrice(), 0)
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

func extractResourceLimiter(res *apigrpc.ResourceLimiter) *types.ResourceLimiter {
	r := &types.ResourceLimiter{
		MinLimits: res.MinLimits,
		MaxLimits: res.MaxLimits,
	}

	return r
}

func extractAvailableMachineTypes(availableMachineTypes *apigrpc.AvailableMachineTypes) []string {
	r := make([]string, len(availableMachineTypes.MachineType))

	copy(r, availableMachineTypes.MachineType)

	sort.Strings(r)

	return r
}

func newTestServer(addNodeGroup, addTestNode bool, desiredState ...AutoScalerServerNodeState) (*AutoScalerServerApp, *AutoScalerServerNodeGroup, context.Context, error) {

	config, ng, kubeclient, err := newTestNodeGroup()

	if err == nil {
		s := &AutoScalerServerApp{
			ResourceLimiter: &types.ResourceLimiter{
				MinLimits: map[string]int64{constantes.ResourceNameCores: 1, constantes.ResourceNameMemory: 10000000},
				MaxLimits: map[string]int64{constantes.ResourceNameCores: 5, constantes.ResourceNameMemory: 100000000},
			},
			Groups:        map[string]*AutoScalerServerNodeGroup{},
			kubeClient:    kubeclient,
			configuration: config,
		}

		if addNodeGroup {
			s.Groups[ng.NodeGroupIdentifier] = ng

			if addTestNode {
				node := createTestNode(ng, testNodeName, desiredState...)

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

func TestServer_NodeGroups(t *testing.T) {
	test := serverTest{t: t}

	test.NodeGroups()
}

func TestServer_NodeGroupForNode(t *testing.T) {
	test := serverTest{t: t}

	test.NodeGroupForNode()

}

func TestServer_Pricing(t *testing.T) {
	test := serverTest{t: t}

	test.Pricing()
}

func TestServer_GetAvailableMachineTypes(t *testing.T) {
	test := serverTest{t: t}

	test.GetAvailableMachineTypes()
}

func TestServer_NewNodeGroup(t *testing.T) {
	test := serverTest{t: t}

	test.NewNodeGroup()
}

func TestServer_GetResourceLimiter(t *testing.T) {
	test := serverTest{t: t}

	test.GetResourceLimiter()
}

func TestServer_Cleanup(t *testing.T) {
	test := serverTest{t: t}

	test.Cleanup()
}

func TestServer_Refresh(t *testing.T) {
	test := serverTest{t: t}

	test.Refresh()
}

func TestServer_MaxSize(t *testing.T) {
	test := serverTest{t: t}

	test.MaxSize()
}

func TestServer_MinSize(t *testing.T) {
	test := serverTest{t: t}

	test.MinSize()
}

func TestServer_TargetSize(t *testing.T) {
	test := serverTest{t: t}

	test.TargetSize()
}

func TestServer_IncreaseSize(t *testing.T) {
	test := serverTest{t: t}

	test.IncreaseSize()
}

func TestServer_DecreaseTargetSize(t *testing.T) {
	test := serverTest{t: t}

	test.DecreaseTargetSize()
}

func TestServer_DeleteNodes(t *testing.T) {
	test := serverTest{t: t}

	test.DeleteNodes()
}

func TestServer_Id(t *testing.T) {
	test := serverTest{t: t}

	test.Id()
}

func TestServer_Debug(t *testing.T) {
	test := serverTest{t: t}

	test.Debug()
}

func TestServer_Nodes(t *testing.T) {
	test := serverTest{t: t}

	test.Nodes()
}

func TestServer_TemplateNodeInfo(t *testing.T) {
	test := serverTest{t: t}

	test.TemplateNodeInfo()
}

func TestServer_Exist(t *testing.T) {
	test := serverTest{t: t}

	test.Exist()
}

func TestServer_Create(t *testing.T) {
	test := serverTest{t: t}

	test.Create()
}

func TestServer_Delete(t *testing.T) {
	test := serverTest{t: t}

	test.Delete()
}

func TestServer_Autoprovisioned(t *testing.T) {
	test := serverTest{t: t}

	test.Autoprovisioned()
}

func TestServer_Belongs(t *testing.T) {
	test := serverTest{t: t}

	test.Belongs()
}

func TestServer_NodePrice(t *testing.T) {
	test := serverTest{t: t}

	test.NodePrice()
}

func TestServer_PodPrice(t *testing.T) {
	test := serverTest{t: t}

	test.PodPrice()
}
