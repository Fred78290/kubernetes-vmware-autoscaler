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

type autoScalerServerAppTest struct {
	AutoScalerServerApp
	ng *autoScalerServerNodeGroupTest
}

func (s *autoScalerServerAppTest) createFakeNode(nodeName string) apiv1.Node {
	return apiv1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
			UID:  testCRDUID,
			Annotations: map[string]string{
				constantes.AnnotationNodeGroupName:        testGroupID,
				constantes.AnnotationNodeIndex:            "0",
				constantes.AnnotationInstanceID:           findInstanceID(s.ng.testConfig, nodeName),
				constantes.AnnotationNodeAutoProvisionned: "true",
			},
		},
	}
}

type serverTest struct {
	baseTest
}

func (m *serverTest) NodeGroups() {
	s, err := m.newTestServer(true, false)

	expected := []string{
		testGroupID,
	}

	if assert.NoError(m.t, err) {
		request := &apigrpc.CloudProviderServiceRequest{
			ProviderID: s.configuration.ServiceIdentifier,
		}

		if got, err := s.NodeGroups(context.TODO(), request); err != nil {
			m.t.Errorf("AutoScalerServerApp.NodeGroups() error = %v", err)
		} else if !reflect.DeepEqual(m.extractNodeGroup(got.GetNodeGroups()), expected) {
			m.t.Errorf("AutoScalerServerApp.NodeGroups() = %v, want %v", m.extractNodeGroup(got.GetNodeGroups()), expected)
		}
	}
}

func (m *serverTest) NodeGroupForNode() {
	s, err := m.newTestServer(true, true)

	if assert.NoError(m.t, err) {
		request := &apigrpc.NodeGroupForNodeRequest{
			ProviderID: s.configuration.ServiceIdentifier,
			Node:       utils.ToJSON(s.createFakeNode(testNodeName)),
		}

		if got, err := s.NodeGroupForNode(context.TODO(), request); err != nil {
			m.t.Errorf("AutoScalerServerApp.NodeGroupForNode() error = %v", err)
		} else if got.GetError() != nil {
			m.t.Errorf("AutoScalerServerApp.NodeGroupForNode() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
		} else if !reflect.DeepEqual(got.GetNodeGroup().GetId(), testGroupID) {
			m.t.Errorf("AutoScalerServerApp.NodeGroupForNode() = %v, want %v", got.GetNodeGroup().GetId(), testGroupID)
		}
	}
}

func (m *serverTest) Pricing() {
	s, err := m.newTestServer(true, false)

	if assert.NoError(m.t, err) {
		request := &apigrpc.CloudProviderServiceRequest{
			ProviderID: s.configuration.ServiceIdentifier,
		}

		if got, err := s.Pricing(context.TODO(), request); err != nil {
			m.t.Errorf("AutoScalerServerApp.Pricing() error = %v", err)
		} else if got.GetError() != nil {
			m.t.Errorf("AutoScalerServerApp.Pricing() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
		} else if !reflect.DeepEqual(got.GetPriceModel().GetId(), s.configuration.ServiceIdentifier) {
			m.t.Errorf("AutoScalerServerApp.Pricing() = %v, want %v", got.GetPriceModel().GetId(), s.configuration.ServiceIdentifier)
		}
	}
}

func (m *serverTest) GetAvailableMachineTypes() {
	s, err := m.newTestServer(true, false)

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
			ProviderID: s.configuration.ServiceIdentifier,
		}

		if got, err := s.GetAvailableMachineTypes(context.TODO(), request); err != nil {
			m.t.Errorf("AutoScalerServerApp.GetAvailableMachineTypes() error = %v", err)
		} else if got.GetError() != nil {
			m.t.Errorf("AutoScalerServerApp.GetAvailableMachineTypes() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
		} else if !reflect.DeepEqual(m.extractAvailableMachineTypes(got.GetAvailableMachineTypes()), expected) {
			m.t.Errorf("AutoScalerServerApp.GetAvailableMachineTypes() = %v, want %v", m.extractAvailableMachineTypes(got.GetAvailableMachineTypes()), expected)
		}
	}
}

func (m *serverTest) NewNodeGroup() {
	s, err := m.newTestServer(false, false)

	if assert.NoError(m.t, err) {
		request := &apigrpc.NewNodeGroupRequest{
			ProviderID:  s.configuration.ServiceIdentifier,
			MachineType: s.configuration.DefaultMachineType,
			Labels:      s.configuration.NodeLabels,
		}

		if got, err := s.NewNodeGroup(context.TODO(), request); err != nil {
			m.t.Errorf("AutoScalerServerApp.NewNodeGroup() error = %v", err)
		} else if got.GetError() != nil {
			m.t.Errorf("AutoScalerServerApp.NewNodeGroup() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
		} else {
			m.t.Logf("AutoScalerServerApp.NewNodeGroup() return node group created :%v", got.GetNodeGroup().GetId())
		}
	}
}

func (m *serverTest) GetResourceLimiter() {
	s, err := m.newTestServer(true, false)

	expected := &types.ResourceLimiter{
		MinLimits: map[string]int64{constantes.ResourceNameCores: 1, constantes.ResourceNameMemory: 10000000},
		MaxLimits: map[string]int64{constantes.ResourceNameCores: 5, constantes.ResourceNameMemory: 100000000},
	}

	if assert.NoError(m.t, err) {
		request := &apigrpc.CloudProviderServiceRequest{
			ProviderID: s.configuration.ServiceIdentifier,
		}

		if got, err := s.GetResourceLimiter(context.TODO(), request); err != nil {
			m.t.Errorf("AutoScalerServerApp.GetResourceLimiter() error = %v", err)
		} else if got.GetError() != nil {
			m.t.Errorf("AutoScalerServerApp.GetResourceLimiter() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
		} else if !reflect.DeepEqual(m.extractResourceLimiter(got.GetResourceLimiter()), expected) {
			m.t.Errorf("AutoScalerServerApp.GetResourceLimiter() = %v, want %v", got.GetResourceLimiter(), expected)
		}
	}
}

func (m *serverTest) Cleanup() {
	s, err := m.newTestServer(true, false)

	if assert.NoError(m.t, err) {
		request := &apigrpc.CloudProviderServiceRequest{
			ProviderID: s.configuration.ServiceIdentifier,
		}

		if got, err := s.Cleanup(context.TODO(), request); err != nil {
			m.t.Errorf("AutoScalerServerApp.Cleanup() error = %v", err)
		} else if got.GetError() != nil && strings.HasSuffix(got.GetError().GetReason(), "is not provisionned by me") == false {
			m.t.Errorf("AutoScalerServerApp.Cleanup() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
		}
	}
}

func (m *serverTest) Refresh() {
	s, err := m.newTestServer(true, true)

	if assert.NoError(m.t, err) {
		request := &apigrpc.CloudProviderServiceRequest{
			ProviderID: s.configuration.ServiceIdentifier,
		}

		if got, err := s.Refresh(context.TODO(), request); err != nil {
			m.t.Errorf("AutoScalerServerApp.Refresh() error = %v", err)
		} else if got.GetError() != nil {
			m.t.Errorf("AutoScalerServerApp.Refresh() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
		}
	}
}

func (m *serverTest) MaxSize() {
	s, err := m.newTestServer(true, false)

	if assert.NoError(m.t, err) {
		request := &apigrpc.NodeGroupServiceRequest{
			ProviderID:  s.configuration.ServiceIdentifier,
			NodeGroupID: testGroupID,
		}

		if got, err := s.MaxSize(context.TODO(), request); err != nil {
			m.t.Errorf("AutoScalerServerApp.MaxSize() error = %v", err)
		} else if got.GetMaxSize() != int32(s.configuration.MaxNode) {
			m.t.Errorf("AutoScalerServerApp.MaxSize() = %v, want %v", got.GetMaxSize(), s.configuration.MaxNode)
		}
	}
}

func (m *serverTest) MinSize() {
	s, err := m.newTestServer(true, false)

	if assert.NoError(m.t, err) {
		request := &apigrpc.NodeGroupServiceRequest{
			ProviderID:  s.configuration.ServiceIdentifier,
			NodeGroupID: testGroupID,
		}

		if got, err := s.MinSize(context.TODO(), request); err != nil {
			m.t.Errorf("AutoScalerServerApp.MinSize() error = %v", err)
		} else if got.GetMinSize() != int32(s.configuration.MinNode) {
			m.t.Errorf("AutoScalerServerApp.MinSize() = %v, want %v", got.GetMinSize(), s.configuration.MinNode)
		}
	}
}

func (m *serverTest) TargetSize() {
	s, err := m.newTestServer(true, true)

	if assert.NoError(m.t, err) {
		request := &apigrpc.NodeGroupServiceRequest{
			ProviderID:  s.configuration.ServiceIdentifier,
			NodeGroupID: testGroupID,
		}

		if got, err := s.TargetSize(context.TODO(), request); err != nil {
			m.t.Errorf("AutoScalerServerApp.TargetSize() error = %v", err)
		} else if got.GetError() != nil {
			m.t.Errorf("AutoScalerServerApp.TargetSize() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
		} else if got.GetTargetSize() != 1 {
			m.t.Errorf("AutoScalerServerApp.TargetSize() = %v, want %v", got.GetTargetSize(), 1)
		}
	}
}

func (m *serverTest) IncreaseSize() {
	s, err := m.newTestServer(true, false)

	if assert.NoError(m.t, err) {
		request := &apigrpc.IncreaseSizeRequest{
			ProviderID:  s.configuration.ServiceIdentifier,
			NodeGroupID: testGroupID,
			Delta:       1,
		}

		if got, err := s.IncreaseSize(context.TODO(), request); err != nil {
			m.t.Errorf("AutoScalerServerApp.IncreaseSize() error = %v", err)
		} else if got.GetError() != nil {
			m.t.Errorf("AutoScalerServerApp.IncreaseSize() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
		}
	}
}

func (m *serverTest) DeleteNodes() {
	s, err := m.newTestServer(true, true)

	if assert.NoError(m.t, err) {
		nodes := []string{utils.ToJSON(s.createFakeNode(testNodeName))}
		request := &apigrpc.DeleteNodesRequest{
			ProviderID:  s.configuration.ServiceIdentifier,
			NodeGroupID: testGroupID,
			Node:        nodes,
		}

		if got, err := s.DeleteNodes(context.TODO(), request); err != nil {
			m.t.Errorf("AutoScalerServerApp.DeleteNodes() error = %v", err)
		} else if got.GetError() != nil {
			m.t.Errorf("AutoScalerServerApp.DeleteNodes() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
		}
	}
}

func (m *serverTest) DecreaseTargetSize() {
	s, err := m.newTestServer(true, true)

	if assert.NoError(m.t, err) {
		request := &apigrpc.DecreaseTargetSizeRequest{
			ProviderID:  s.configuration.ServiceIdentifier,
			NodeGroupID: testGroupID,
			Delta:       -1,
		}

		if got, err := s.DecreaseTargetSize(context.TODO(), request); err != nil {
			m.t.Errorf("AutoScalerServerApp.DecreaseTargetSize() error = %v", err)
		} else if got.GetError() != nil && !strings.HasPrefix(got.GetError().GetReason(), "attempt to delete existing nodes") {
			m.t.Errorf("AutoScalerServerApp.DecreaseTargetSize() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
		}
	}
}

func (m *serverTest) Id() {
	s, err := m.newTestServer(true, false)

	if assert.NoError(m.t, err) {
		request := &apigrpc.NodeGroupServiceRequest{
			ProviderID:  s.configuration.ServiceIdentifier,
			NodeGroupID: testGroupID,
		}

		if got, err := s.Id(context.TODO(), request); err != nil {
			m.t.Errorf("AutoScalerServerApp.Id() error = %v", err)
		} else if got.GetResponse() != testGroupID {
			m.t.Errorf("AutoScalerServerApp.Id() = %v, want %v", got, testGroupID)
		}
	}
}

func (m *serverTest) Debug() {
	s, err := m.newTestServer(true, false)

	if assert.NoError(m.t, err) {
		request := &apigrpc.NodeGroupServiceRequest{
			ProviderID:  s.configuration.ServiceIdentifier,
			NodeGroupID: testGroupID,
		}

		if _, err := s.Debug(context.TODO(), request); err != nil {
			m.t.Errorf("AutoScalerServerApp.Debug() error = %v", err)
		}
	}
}

func (m *serverTest) Nodes() {
	s, err := m.newTestServer(true, true)

	expected := []string{
		fmt.Sprintf("vsphere://%s", testVMUUID),
	}

	if assert.NoError(m.t, err) {
		request := &apigrpc.NodeGroupServiceRequest{
			ProviderID:  s.configuration.ServiceIdentifier,
			NodeGroupID: testGroupID,
		}

		if got, err := s.Nodes(context.TODO(), request); err != nil {
			m.t.Errorf("AutoScalerServerApp.Nodes() error = %v", err)
		} else if got.GetError() != nil {
			m.t.Errorf("AutoScalerServerApp.Nodes() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
		} else if !reflect.DeepEqual(m.extractInstanceID(got.GetInstances()), expected) {
			m.t.Errorf("AutoScalerServerApp.Nodes() = %v, want %v", m.extractInstanceID(got.GetInstances()), expected)
		}
	}
}

func (m *serverTest) TemplateNodeInfo() {
	s, err := m.newTestServer(true, true)

	if assert.NoError(m.t, err) {
		request := &apigrpc.NodeGroupServiceRequest{
			ProviderID:  s.configuration.ServiceIdentifier,
			NodeGroupID: testGroupID,
		}

		if got, err := s.TemplateNodeInfo(context.TODO(), request); err != nil {
			m.t.Errorf("AutoScalerServerApp.TemplateNodeInfo() error = %v", err)
		} else if got.GetError() != nil {
			m.t.Errorf("AutoScalerServerApp.TemplateNodeInfo() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
		}
	}
}

func (m *serverTest) Exist() {
	s, err := m.newTestServer(true, false)

	if assert.NoError(m.t, err) {
		request := &apigrpc.NodeGroupServiceRequest{
			ProviderID:  s.configuration.ServiceIdentifier,
			NodeGroupID: testGroupID,
		}

		if got, err := s.Exist(context.TODO(), request); err != nil {
			m.t.Errorf("AutoScalerServerApp.Exist() error = %v", err)
		} else if got.GetExists() == false {
			m.t.Errorf("AutoScalerServerApp.Exist() = %v", got.GetExists())
		}
	}
}

func (m *serverTest) Create() {
	s, err := m.newTestServer(true, false)

	if assert.NoError(m.t, err) {
		request := &apigrpc.NodeGroupServiceRequest{
			ProviderID:  s.configuration.ServiceIdentifier,
			NodeGroupID: testGroupID,
		}

		if got, err := s.Create(context.TODO(), request); err != nil {
			m.t.Errorf("AutoScalerServerApp.Create() error = %v", err)
		} else if got.GetError() != nil {
			m.t.Errorf("AutoScalerServerApp.Create() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
		} else if got.GetNodeGroup().GetId() != testGroupID {
			m.t.Errorf("AutoScalerServerApp.Create() = %v, want %v", got.GetNodeGroup().GetId(), testGroupID)
		}
	}
}

func (m *serverTest) Delete() {
	s, err := m.newTestServer(true, false)

	if assert.NoError(m.t, err) {
		request := &apigrpc.NodeGroupServiceRequest{
			ProviderID:  s.configuration.ServiceIdentifier,
			NodeGroupID: testGroupID,
		}

		if got, err := s.Delete(context.TODO(), request); err != nil {
			m.t.Errorf("AutoScalerServerApp.Delete() error = %v", err)
		} else if got.GetError() != nil {
			m.t.Errorf("AutoScalerServerApp.Delete() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
		}
	}
}

func (m *serverTest) Autoprovisioned() {
	s, err := m.newTestServer(true, false)

	if assert.NoError(m.t, err) {
		request := &apigrpc.NodeGroupServiceRequest{
			ProviderID:  s.configuration.ServiceIdentifier,
			NodeGroupID: testGroupID,
		}

		if got, err := s.Autoprovisioned(context.TODO(), request); err != nil {
			m.t.Errorf("AutoScalerServerApp.Autoprovisioned() error = %v", err)
		} else if got.GetAutoprovisioned() == false {
			m.t.Errorf("AutoScalerServerApp.Autoprovisioned() = %v, want true", got.GetAutoprovisioned())
		}
	}
}

func (m *serverTest) Belongs() {
	s, err := m.newTestServer(true, true)

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
				ProviderID:  s.configuration.ServiceIdentifier,
				NodeGroupID: testGroupID,
				Node:        utils.ToJSON(s.createFakeNode(testNodeName)),
			},
		},
		{
			name:    "NotBelongs",
			want:    false,
			wantErr: false,
			request: &apigrpc.BelongsRequest{
				ProviderID:  s.configuration.ServiceIdentifier,
				NodeGroupID: testGroupID,
				Node:        utils.ToJSON(s.createFakeNode("wrong-name")),
			},
		},
	}

	if assert.NoError(m.t, err) {
		for _, test := range tests {

			got, err := s.Belongs(context.TODO(), test.request)

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
	s, err := m.newTestServer(true, true)

	if assert.NoError(m.t, err) {
		request := &apigrpc.NodePriceRequest{
			ProviderID: s.configuration.ServiceIdentifier,
			StartTime:  time.Now().Unix(),
			EndTime:    time.Now().Add(time.Hour).Unix(),
			Node:       utils.ToJSON(s.createFakeNode(testNodeName)),
		}

		if got, err := s.NodePrice(context.TODO(), request); err != nil {
			m.t.Errorf("AutoScalerServerApp.NodePrice() error = %v", err)
		} else if got.GetError() != nil {
			m.t.Errorf("AutoScalerServerApp.NodePrice() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
		} else if got.GetPrice() != 0 {
			m.t.Errorf("AutoScalerServerApp.NodePrice() = %v, want %v", got.GetPrice(), 0)
		}
	}
}

func (m *serverTest) PodPrice() {
	s, err := m.newTestServer(true, true)

	if assert.NoError(m.t, err) {
		request := &apigrpc.PodPriceRequest{
			ProviderID: s.configuration.ServiceIdentifier,
			StartTime:  time.Now().Unix(),
			EndTime:    time.Now().Add(time.Hour).Unix(),
			Pod:        utils.ToJSON(s.createFakeNode(testNodeName)),
		}

		if got, err := s.PodPrice(context.TODO(), request); err != nil {
			m.t.Errorf("AutoScalerServerApp.PodPrice() error = %v", err)
		} else if got.GetError() != nil {
			m.t.Errorf("AutoScalerServerApp.PodPrice() return an error, code = %v, reason = %s", got.GetError().GetCode(), got.GetError().GetReason())
		} else if got.GetPrice() != 0 {
			m.t.Errorf("AutoScalerServerApp.PodPrice() = %v, want %v", got.GetPrice(), 0)
		}
	}
}

func (m *serverTest) extractInstanceID(instances *apigrpc.Instances) []string {
	r := make([]string, len(instances.GetItems()))

	for i, n := range instances.GetItems() {
		r[i] = n.GetId()
	}

	return r
}

func (m *serverTest) extractNodeGroup(nodeGroups []*apigrpc.NodeGroup) []string {
	r := make([]string, len(nodeGroups))

	for i, n := range nodeGroups {
		r[i] = n.Id
	}

	return r
}

func (m *serverTest) extractResourceLimiter(res *apigrpc.ResourceLimiter) *types.ResourceLimiter {
	r := &types.ResourceLimiter{
		MinLimits: res.MinLimits,
		MaxLimits: res.MaxLimits,
	}

	return r
}

func (m *serverTest) extractAvailableMachineTypes(availableMachineTypes *apigrpc.AvailableMachineTypes) []string {
	r := make([]string, len(availableMachineTypes.MachineType))

	copy(r, availableMachineTypes.MachineType)

	sort.Strings(r)

	return r
}

func (m *serverTest) newTestServer(addNodeGroup, addTestNode bool, desiredState ...AutoScalerServerNodeState) (*autoScalerServerAppTest, error) {

	if ng, err := m.newTestNodeGroup(); err == nil {
		s := &autoScalerServerAppTest{
			ng: ng,
			AutoScalerServerApp: AutoScalerServerApp{
				ResourceLimiter: &types.ResourceLimiter{
					MinLimits: map[string]int64{constantes.ResourceNameCores: 1, constantes.ResourceNameMemory: 10000000},
					MaxLimits: map[string]int64{constantes.ResourceNameCores: 5, constantes.ResourceNameMemory: 100000000},
				},
				Groups:        map[string]*AutoScalerServerNodeGroup{},
				kubeClient:    m,
				configuration: ng.configuration,
			},
		}

		if addNodeGroup {
			s.Groups[ng.NodeGroupIdentifier] = &ng.AutoScalerServerNodeGroup

			if addTestNode {
				ng.createTestNode(testNodeName, desiredState...)
			}
		}

		return s, nil
	} else {
		return nil, err
	}
}

func createServerTest(t *testing.T) *serverTest {
	return &serverTest{
		baseTest: baseTest{
			t: t,
		},
	}
}

func TestServer_NodeGroups(t *testing.T) {
	createServerTest(t).NodeGroups()
}

func TestServer_NodeGroupForNode(t *testing.T) {
	createServerTest(t).NodeGroupForNode()
}

func TestServer_Pricing(t *testing.T) {
	createServerTest(t).Pricing()
}

func TestServer_GetAvailableMachineTypes(t *testing.T) {
	createServerTest(t).GetAvailableMachineTypes()
}

func TestServer_NewNodeGroup(t *testing.T) {
	createServerTest(t).NewNodeGroup()
}

func TestServer_GetResourceLimiter(t *testing.T) {
	createServerTest(t).GetResourceLimiter()
}

func TestServer_Cleanup(t *testing.T) {
	createServerTest(t).Cleanup()
}

func TestServer_Refresh(t *testing.T) {
	createServerTest(t).Refresh()
}

func TestServer_MaxSize(t *testing.T) {
	createServerTest(t).MaxSize()
}

func TestServer_MinSize(t *testing.T) {
	createServerTest(t).MinSize()
}

func TestServer_TargetSize(t *testing.T) {
	createServerTest(t).TargetSize()
}

func TestServer_IncreaseSize(t *testing.T) {
	createServerTest(t).IncreaseSize()
}

func TestServer_DecreaseTargetSize(t *testing.T) {
	createServerTest(t).DecreaseTargetSize()
}

func TestServer_DeleteNodes(t *testing.T) {
	createServerTest(t).DeleteNodes()
}

func TestServer_Id(t *testing.T) {
	createServerTest(t).Id()
}

func TestServer_Debug(t *testing.T) {
	createServerTest(t).Debug()
}

func TestServer_Nodes(t *testing.T) {
	createServerTest(t).Nodes()
}

func TestServer_TemplateNodeInfo(t *testing.T) {
	createServerTest(t).TemplateNodeInfo()
}

func TestServer_Exist(t *testing.T) {
	createServerTest(t).Exist()
}

func TestServer_Create(t *testing.T) {
	createServerTest(t).Create()
}

func TestServer_Delete(t *testing.T) {
	createServerTest(t).Delete()
}

func TestServer_Autoprovisioned(t *testing.T) {
	createServerTest(t).Autoprovisioned()
}

func TestServer_Belongs(t *testing.T) {
	createServerTest(t).Belongs()
}

func TestServer_NodePrice(t *testing.T) {
	createServerTest(t).NodePrice()
}

func TestServer_PodPrice(t *testing.T) {
	createServerTest(t).PodPrice()
}
