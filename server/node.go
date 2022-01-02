package server

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Fred78290/kubernetes-vmware-autoscaler/constantes"
	"github.com/Fred78290/kubernetes-vmware-autoscaler/types"
	"github.com/Fred78290/kubernetes-vmware-autoscaler/utils"
	"github.com/Fred78290/kubernetes-vmware-autoscaler/vsphere"
	glog "github.com/sirupsen/logrus"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	uid "k8s.io/apimachinery/pkg/types"
)

// AutoScalerServerNodeState VM state
type AutoScalerServerNodeState int32

// AutoScalerServerNodeType node class (external, autoscaled, managed)
type AutoScalerServerNodeType int32

// autoScalerServerNodeStateString strings
var autoScalerServerNodeStateString = []string{
	"AutoScalerServerNodeStateNotCreated",
	"AutoScalerServerNodeStateRunning",
	"AutoScalerServerNodeStateStopped",
	"AutoScalerServerNodeStateDeleted",
	"AutoScalerServerNodeStateUndefined",
}

const (
	// AutoScalerServerNodeStateNotCreated not created state
	AutoScalerServerNodeStateNotCreated = iota

	// AutoScalerServerNodeStateRunning running state
	AutoScalerServerNodeStateRunning

	// AutoScalerServerNodeStateStopped stopped state
	AutoScalerServerNodeStateStopped

	// AutoScalerServerNodeStateDeleted deleted state
	AutoScalerServerNodeStateDeleted

	// AutoScalerServerNodeStateUndefined undefined state
	AutoScalerServerNodeStateUndefined
)

const (
	// AutoScalerServerNodeExternal is a node create out of autoscaler
	AutoScalerServerNodeExternal = iota
	// AutoScalerServerNodeAutoscaled is a node create by autoscaler
	AutoScalerServerNodeAutoscaled
	// AutoScalerServerNodeManaged is a node managed by controller
	AutoScalerServerNodeManaged
)

// AutoScalerServerNode Describe a AutoScaler VM
type AutoScalerServerNode struct {
	ProviderID       string                    `json:"providerID"`
	NodeGroupID      string                    `json:"group"`
	NodeName         string                    `json:"name"`
	NodeIndex        int                       `json:"index"`
	UID              uid.UID                   `json:"crd-uid"`
	Memory           int                       `json:"memory"`
	CPU              int                       `json:"cpu"`
	Disk             int                       `json:"disk"`
	IPAddress        string                    `json:"address"`
	State            AutoScalerServerNodeState `json:"state"`
	NodeType         AutoScalerServerNodeType  `json:"type"`
	ControlPlaneNode bool                      `json:"control-plane,omitempty"`
	AllowDeployment  bool                      `json:"allow-deployment,omitempty"`
	ExtraLabels      KubernetesLabel           `json:"labels,omitempty"`
	ExtraAnnotations KubernetesLabel           `json:"annotations,omitempty"`
	VSphereConfig    *vsphere.Configuration    `json:"vmware"`
	serverConfig     *types.AutoScalerServerConfig
}

func (s AutoScalerServerNodeState) String() string {
	return autoScalerServerNodeStateString[s]
}

func (vm *AutoScalerServerNode) waitReady(c types.ClientGenerator) error {
	glog.Debugf("AutoScalerNode::waitReady, node:%s", vm.NodeName)

	return c.WaitNodeToBeReady(vm.NodeName, 60)
}

func (vm *AutoScalerServerNode) recopyEtcdSslFilesIfNeeded() error {
	var err error

	if vm.ControlPlaneNode || *vm.serverConfig.UseExternalEtdc {
		if err = utils.Scp(vm.serverConfig.SSH, vm.IPAddress, vm.serverConfig.ExtSourceEtcdSslDir, "."); err != nil {
			glog.Errorf("scp failed: %v", err)
		} else if _, err = utils.Sudo(vm.serverConfig.SSH, vm.IPAddress, fmt.Sprintf("mkdir -p %s", vm.serverConfig.ExtDestinationEtcdSslDir)); err != nil {
			glog.Errorf("mkdir failed: %v", err)
		} else if _, err = utils.Sudo(vm.serverConfig.SSH, vm.IPAddress, fmt.Sprintf("cp -r %s/* %s", filepath.Base(vm.serverConfig.ExtSourceEtcdSslDir), vm.serverConfig.ExtDestinationEtcdSslDir)); err != nil {
			glog.Errorf("mv failed: %v", err)
		} else if _, err = utils.Sudo(vm.serverConfig.SSH, vm.IPAddress, fmt.Sprintf("chown -R root:root %s", vm.serverConfig.ExtDestinationEtcdSslDir)); err != nil {
			glog.Errorf("chown failed: %v", err)
		}
	}

	return err
}

func (vm *AutoScalerServerNode) recopyKubernetesPKIIfNeeded() error {
	var err error

	if vm.ControlPlaneNode {
		if err = utils.Scp(vm.serverConfig.SSH, vm.IPAddress, vm.serverConfig.KubernetesPKISourceDir, "."); err != nil {
			glog.Errorf("scp failed: %v", err)
		} else if _, err = utils.Sudo(vm.serverConfig.SSH, vm.IPAddress, fmt.Sprintf("mkdir -p %s", vm.serverConfig.KubernetesPKIDestDir)); err != nil {
			glog.Errorf("mkdir failed: %v", err)
		} else if _, err = utils.Sudo(vm.serverConfig.SSH, vm.IPAddress, fmt.Sprintf("cp -r %s/* %s", filepath.Base(vm.serverConfig.KubernetesPKISourceDir), vm.serverConfig.KubernetesPKIDestDir)); err != nil {
			glog.Errorf("mv failed: %v", err)
		} else if _, err = utils.Sudo(vm.serverConfig.SSH, vm.IPAddress, fmt.Sprintf("chown -R root:root %s", vm.serverConfig.KubernetesPKIDestDir)); err != nil {
			glog.Errorf("chown failed: %v", err)
		}
	}

	return err
}

func (vm *AutoScalerServerNode) kubeAdmJoin() error {
	kubeAdm := vm.serverConfig.KubeAdm

	args := []string{
		"kubeadm",
		"join",
		kubeAdm.Address,
		"--token",
		kubeAdm.Token,
		"--discovery-token-ca-cert-hash",
		kubeAdm.CACert,
		"--apiserver-advertise-address",
		vm.IPAddress,
	}

	if vm.ControlPlaneNode {
		args = append(args, "--control-plane")
	}

	// Append extras arguments
	if len(kubeAdm.ExtraArguments) > 0 {
		args = append(args, kubeAdm.ExtraArguments...)
	}

	command := strings.Join(args, " ")

	if out, err := utils.Sudo(vm.serverConfig.SSH, vm.IPAddress, command); err != nil {
		return fmt.Errorf("unable to execute command: %s, output: %s, reason:%v", command, out, err)
	}

	return nil
}

func (vm *AutoScalerServerNode) setNodeLabels(c types.ClientGenerator, nodeLabels, systemLabels KubernetesLabel) error {
	labels := KubernetesLabel{
		constantes.NodeLabelGroupName: vm.NodeGroupID,
	}

	// Append extras arguments
	for k, v := range nodeLabels {
		labels[k] = v
	}

	for k, v := range systemLabels {
		labels[k] = v
	}

	if err := c.LabelNode(vm.NodeName, labels); err != nil {
		return fmt.Errorf(constantes.ErrLabelNodeReturnError, vm.NodeName, err)
	}

	if len(vm.ExtraLabels) > 0 {
		if err := c.LabelNode(vm.NodeName, vm.ExtraLabels); err != nil {
			return fmt.Errorf(constantes.ErrLabelNodeReturnError, vm.NodeName, err)
		}
	}

	annotations := KubernetesLabel{
		constantes.NodeLabelGroupName:             vm.NodeGroupID,
		constantes.AnnotationScaleDownDisabled:    strconv.FormatBool(vm.NodeType == AutoScalerServerNodeManaged),
		constantes.AnnotationNodeAutoProvisionned: strconv.FormatBool(vm.NodeType == AutoScalerServerNodeAutoscaled),
		constantes.AnnotationNodeManaged:          strconv.FormatBool(vm.NodeType == AutoScalerServerNodeManaged),
		constantes.AnnotationNodeIndex:            strconv.Itoa(vm.NodeIndex),
	}

	if err := c.AnnoteNode(vm.NodeName, annotations); err != nil {
		return fmt.Errorf(constantes.ErrAnnoteNodeReturnError, vm.NodeName, err)
	}

	if len(vm.ExtraAnnotations) > 0 {
		if err := c.AnnoteNode(vm.NodeName, vm.ExtraAnnotations); err != nil {
			return fmt.Errorf(constantes.ErrAnnoteNodeReturnError, vm.NodeName, err)
		}

	}

	if vm.ControlPlaneNode && vm.AllowDeployment {
		c.TaintNode(vm.NodeName,
			apiv1.Taint{
				Key:    constantes.NodeLabelControlPlaneRole,
				Effect: apiv1.TaintEffectNoSchedule,
				TimeAdded: &metav1.Time{
					Time: time.Now(),
				},
			},
			apiv1.Taint{
				Key:    constantes.NodeLabelMasterRole,
				Effect: apiv1.TaintEffectNoSchedule,
				TimeAdded: &metav1.Time{
					Time: time.Now(),
				},
			})
	}

	return nil
}

func (vm *AutoScalerServerNode) launchVM(c types.ClientGenerator, nodeLabels, systemLabels KubernetesLabel) error {
	glog.Debugf("AutoScalerNode::launchVM, node:%s", vm.NodeName)

	var err error
	var status AutoScalerServerNodeState
	var hostsystem string

	vsphere := vm.VSphereConfig
	network := vsphere.Network
	userInfo := vm.serverConfig.SSH

	glog.Infof("Launch VM:%s for nodegroup: %s", vm.NodeName, vm.NodeGroupID)

	if vm.NodeType != AutoScalerServerNodeAutoscaled && vm.NodeType != AutoScalerServerNodeManaged {

		err = fmt.Errorf(constantes.ErrVMNotProvisionnedByMe, vm.NodeName)

	} else if vm.State != AutoScalerServerNodeStateNotCreated {

		err = fmt.Errorf(constantes.ErrVMAlreadyCreated, vm.NodeName)

	} else if _, err = vsphere.Create(vm.NodeName, userInfo.GetUserName(), userInfo.GetAuthKeys(), vm.serverConfig.CloudInit, network, "", true, vm.Memory, vm.CPU, vm.Disk, vm.NodeIndex); err != nil {

		err = fmt.Errorf(constantes.ErrUnableToLaunchVM, vm.NodeName, err)

	} else if err = vsphere.PowerOn(vm.NodeName); err != nil {

		err = fmt.Errorf(constantes.ErrStartVMFailed, vm.NodeName, err)

	} else if hostsystem, err = vsphere.GetHostSystem(vm.NodeName); err != nil {

		err = fmt.Errorf(constantes.ErrStartVMFailed, vm.NodeName, err)

	} else if err = vsphere.SetAutoStart(hostsystem, vm.NodeName, -1); err != nil {

		err = fmt.Errorf(constantes.ErrStartVMFailed, vm.NodeName, err)

	} else if _, err = vsphere.WaitForToolsRunning(vm.NodeName); err != nil {

		err = fmt.Errorf(constantes.ErrStartVMFailed, vm.NodeName, err)

	} else if _, err = vsphere.WaitForIP(vm.NodeName); err != nil {

		err = fmt.Errorf(constantes.ErrStartVMFailed, vm.NodeName, err)

	} else if status, err = vm.statusVM(); err != nil {

		err = fmt.Errorf(constantes.ErrGetVMInfoFailed, vm.NodeName, err)

	} else if status != AutoScalerServerNodeStateRunning {

		err = fmt.Errorf(constantes.ErrStartVMFailed, vm.NodeName, err)

	} else if err = vm.recopyKubernetesPKIIfNeeded(); err != nil {

		err = fmt.Errorf(constantes.ErrRecopyKubernetesPKIFailed, vm.NodeName, err)

	} else if err = vm.recopyEtcdSslFilesIfNeeded(); err != nil {

		err = fmt.Errorf(constantes.ErrUpdateEtcdSslFailed, vm.NodeName, err)

	} else if err = vm.kubeAdmJoin(); err != nil {

		err = fmt.Errorf(constantes.ErrKubeAdmJoinFailed, vm.NodeName, err)

	} else if err = c.SetProviderID(vm.NodeName, vm.ProviderID); err != nil {

		err = fmt.Errorf(constantes.ErrProviderIDNotConfigured, vm.NodeName, err)

	} else if err = vm.waitReady(c); err != nil {

		err = fmt.Errorf(constantes.ErrNodeIsNotReady, vm.NodeName)

	} else {
		err = vm.setNodeLabels(c, nodeLabels, systemLabels)
	}

	if err == nil {
		glog.Infof("Launched VM:%s for nodegroup: %s", vm.NodeName, vm.NodeGroupID)
	} else {
		glog.Errorf("Unable to launch VM:%s for nodegroup: %s. Reason: %v", vm.NodeName, vm.NodeGroupID, err.Error())
	}

	return err
}

func (vm *AutoScalerServerNode) startVM(c types.ClientGenerator) error {
	glog.Debugf("AutoScalerNode::startVM, node:%s", vm.NodeName)

	var err error
	var state AutoScalerServerNodeState

	glog.Infof("Start VM:%s", vm.NodeName)

	vsphere := vm.VSphereConfig

	if vm.NodeType != AutoScalerServerNodeAutoscaled && vm.NodeType != AutoScalerServerNodeManaged {

		err = fmt.Errorf(constantes.ErrVMNotProvisionnedByMe, vm.NodeName)

	} else if state, err = vm.statusVM(); err != nil {

		err = fmt.Errorf(constantes.ErrStartVMFailed, vm.NodeName, err)

	} else if state == AutoScalerServerNodeStateStopped {

		if err = vsphere.PowerOn(vm.NodeName); err != nil {

			err = fmt.Errorf(constantes.ErrStartVMFailed, vm.NodeName, err)

		} else if _, err = vsphere.WaitForIP(vm.NodeName); err != nil {

			err = fmt.Errorf(constantes.ErrStartVMFailed, vm.NodeName, err)

		} else if state, err = vm.statusVM(); err != nil {

			err = fmt.Errorf(constantes.ErrStartVMFailed, vm.NodeName, err)

		} else if state != AutoScalerServerNodeStateRunning {

			err = fmt.Errorf(constantes.ErrStartVMFailed, vm.NodeName, err)

		} else {
			if err = c.UncordonNode(vm.NodeName); err != nil {
				glog.Errorf(constantes.ErrUncordonNodeReturnError, vm.NodeName, err)

				err = nil
			}

			vm.State = AutoScalerServerNodeStateRunning
		}
	} else if state != AutoScalerServerNodeStateRunning {
		err = fmt.Errorf(constantes.ErrStartVMFailed, vm.NodeName, fmt.Sprintf("Unexpected state: %d", state))
	}

	if err == nil {
		glog.Infof("Started VM:%s", vm.NodeName)
	} else {
		glog.Errorf("Unable to start VM:%s. Reason: %v", vm.NodeName, err)
	}

	return err
}

func (vm *AutoScalerServerNode) stopVM(c types.ClientGenerator) error {
	glog.Debugf("AutoScalerNode::stopVM, node:%s", vm.NodeName)

	var err error
	var state AutoScalerServerNodeState

	glog.Infof("Stop VM:%s", vm.NodeName)

	vsphere := vm.VSphereConfig

	if vm.NodeType != AutoScalerServerNodeAutoscaled && vm.NodeType != AutoScalerServerNodeManaged {

		err = fmt.Errorf(constantes.ErrVMNotProvisionnedByMe, vm.NodeName)

	} else if state, err = vm.statusVM(); err != nil {

		err = fmt.Errorf(constantes.ErrStopVMFailed, vm.NodeName, err)

	} else if state == AutoScalerServerNodeStateRunning {
		if err = c.CordonNode(vm.NodeName); err != nil {
			glog.Errorf(constantes.ErrCordonNodeReturnError, vm.NodeName, err)
		}

		if err = vsphere.PowerOff(vm.NodeName); err == nil {
			vm.State = AutoScalerServerNodeStateStopped
		} else {
			err = fmt.Errorf(constantes.ErrStopVMFailed, vm.NodeName, err)
		}

	} else if state != AutoScalerServerNodeStateStopped {

		err = fmt.Errorf(constantes.ErrStopVMFailed, vm.NodeName, fmt.Sprintf("Unexpected state: %d", state))

	}

	if err == nil {
		glog.Infof("Stopped VM:%s", vm.NodeName)
	} else {
		glog.Errorf("Could not stop VM:%s. Reason: %s", vm.NodeName, err)
	}

	return err
}

func (vm *AutoScalerServerNode) deleteVM(c types.ClientGenerator) error {
	glog.Debugf("AutoScalerNode::deleteVM, node:%s", vm.NodeName)

	var err error
	var status *vsphere.Status

	if vm.NodeType != AutoScalerServerNodeAutoscaled && vm.NodeType != AutoScalerServerNodeManaged {
		err = fmt.Errorf(constantes.ErrVMNotProvisionnedByMe, vm.NodeName)
	} else {
		vsphere := vm.VSphereConfig

		if status, err = vsphere.Status(vm.NodeName); err == nil {
			if status.Powered {
				if err = c.MarkDrainNode(vm.NodeName); err != nil {
					glog.Errorf(constantes.ErrCordonNodeReturnError, vm.NodeName, err)
				}

				if err = c.DrainNode(vm.NodeName, true, true); err != nil {
					glog.Errorf(constantes.ErrDrainNodeReturnError, vm.NodeName, err)
				}

				if err = c.DeleteNode(vm.NodeName); err != nil {
					glog.Errorf(constantes.ErrDeleteNodeReturnError, vm.NodeName, err)
				}

				if err = vsphere.PowerOff(vm.NodeName); err != nil {
					err = fmt.Errorf(constantes.ErrStopVMFailed, vm.NodeName, err)
				} else {
					vm.State = AutoScalerServerNodeStateStopped

					if err = vsphere.Delete(vm.NodeName); err != nil {
						err = fmt.Errorf(constantes.ErrDeleteVMFailed, vm.NodeName, err)
					}
				}
			} else if err = vsphere.Delete(vm.NodeName); err != nil {
				err = fmt.Errorf(constantes.ErrDeleteVMFailed, vm.NodeName, err)
			}
		}
	}

	if err == nil {
		glog.Infof("Deleted VM:%s", vm.NodeName)
		vm.State = AutoScalerServerNodeStateDeleted
	} else {
		glog.Errorf("Could not delete VM:%s. Reason: %s", vm.NodeName, err)
	}

	return err
}

func (vm *AutoScalerServerNode) statusVM() (AutoScalerServerNodeState, error) {
	glog.Debugf("AutoScalerNode::statusVM, node:%s", vm.NodeName)

	// Get VM infos
	var status *vsphere.Status
	var err error

	if status, err = vm.VSphereConfig.Status(vm.NodeName); err != nil {
		glog.Errorf(constantes.ErrGetVMInfoFailed, vm.NodeName, err)
		return AutoScalerServerNodeStateUndefined, err
	}

	if status != nil {
		vm.IPAddress = vm.VSphereConfig.FindPreferredIPAddress(status.Interfaces)

		if status.Powered {
			vm.State = AutoScalerServerNodeStateRunning
		} else {
			vm.State = AutoScalerServerNodeStateStopped
		}

		return vm.State, nil
	}

	return AutoScalerServerNodeStateUndefined, fmt.Errorf(constantes.ErrAutoScalerInfoNotFound, vm.NodeName)
}

// GetVSphere method
func (vm *AutoScalerServerNode) GetVSphere() *vsphere.Configuration {
	var vsphere *vsphere.Configuration

	if vsphere = vm.serverConfig.VMwareInfos[vm.NodeGroupID]; vsphere == nil {
		vsphere = vm.serverConfig.VMwareInfos["default"]
	}

	if vsphere == nil {
		glog.Fatalf("Unable to find vmware config for node:%s in group:%s", vm.NodeName, vm.NodeGroupID)
	}

	return vsphere
}

func (vm *AutoScalerServerNode) setServerConfiguration(config *types.AutoScalerServerConfig) {
	vm.VSphereConfig.Network.UpdateMacAddressTable(vm.NodeIndex)
	vm.serverConfig = config
}

func (vm *AutoScalerServerNode) retrieveNetworkInfos() error {
	return vm.VSphereConfig.RetrieveNetworkInfos(vm.NodeName, vm.NodeIndex)
}
