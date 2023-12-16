package server

import (
	"fmt"
	"os"
	"path"
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
	"AutoScalerServerNodeStateCreating",
	"AutoScalerServerNodeStateRunning",
	"AutoScalerServerNodeStateStopped",
	"AutoScalerServerNodeStateDeleted",
	"AutoScalerServerNodeStateUndefined",
}

const (
	// AutoScalerServerNodeStateNotCreated not created state
	AutoScalerServerNodeStateNotCreated = iota

	// AutoScalerServerNodeStateCreating running state
	AutoScalerServerNodeStateCreating

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
	NodeGroupID      string                    `json:"group"`
	NodeName         string                    `json:"name"`
	NodeIndex        int                       `json:"index"`
	VMUUID           string                    `json:"vm-uuid"`
	CRDUID           uid.UID                   `json:"crd-uid"`
	Memory           int                       `json:"memory"`
	CPU              int                       `json:"cpu"`
	Disk             int                       `json:"disk"`
	IPAddress        string                    `json:"address"`
	State            AutoScalerServerNodeState `json:"state"`
	NodeType         AutoScalerServerNodeType  `json:"type"`
	ControlPlaneNode bool                      `json:"control-plane,omitempty"`
	AllowDeployment  bool                      `json:"allow-deployment,omitempty"`
	ExtraLabels      types.KubernetesLabel     `json:"labels,omitempty"`
	ExtraAnnotations types.KubernetesLabel     `json:"annotations,omitempty"`
	VSphereConfig    *vsphere.Configuration    `json:"vmware"`
	serverConfig     *types.AutoScalerServerConfig
}

const (
	joinClusterInfo         = "Join cluster for node:%s for nodegroup: %s"
	unableToExecuteCmdError = "unable to execute command: %s, output: %s, reason:%v"
	nodeNameTemplate        = "%s-%s-%02d"
)

func (s AutoScalerServerNodeState) String() string {
	return autoScalerServerNodeStateString[s]
}

func (vm *AutoScalerServerNode) waitReady(c types.ClientGenerator) error {
	glog.Debugf("AutoScalerNode::waitReady, node:%s", vm.NodeName)

	return c.WaitNodeToBeReady(vm.NodeName)
}

func (vm *AutoScalerServerNode) recopyEtcdSslFilesIfNeeded() error {
	var err error

	if vm.ControlPlaneNode || *vm.serverConfig.UseExternalEtdc {
		glog.Infof("Recopy etcd certs for node:%s for nodegroup: %s", vm.NodeName, vm.NodeGroupID)

		if err = utils.Scp(vm.serverConfig.SSH, vm.IPAddress, vm.serverConfig.ExtSourceEtcdSslDir, "."); err != nil {
			glog.Errorf("scp failed: %v", err)
		} else if _, err = utils.Sudo(vm.serverConfig.SSH, vm.IPAddress, vm.VSphereConfig.Timeout, fmt.Sprintf("mkdir -p %s", vm.serverConfig.ExtDestinationEtcdSslDir)); err != nil {
			glog.Errorf("mkdir failed: %v", err)
		} else if _, err = utils.Sudo(vm.serverConfig.SSH, vm.IPAddress, vm.VSphereConfig.Timeout, fmt.Sprintf("cp -r %s/* %s", filepath.Base(vm.serverConfig.ExtSourceEtcdSslDir), vm.serverConfig.ExtDestinationEtcdSslDir)); err != nil {
			glog.Errorf("mv failed: %v", err)
		} else if _, err = utils.Sudo(vm.serverConfig.SSH, vm.IPAddress, vm.VSphereConfig.Timeout, fmt.Sprintf("chown -R root:root %s", vm.serverConfig.ExtDestinationEtcdSslDir)); err != nil {
			glog.Errorf("chown failed: %v", err)
		}
	}

	return err
}

func (vm *AutoScalerServerNode) recopyKubernetesPKIIfNeeded() error {
	var err error

	if vm.ControlPlaneNode {
		glog.Infof("Recopy pki kubernetes certs for node:%s for nodegroup: %s", vm.NodeName, vm.NodeGroupID)

		if err = utils.Scp(vm.serverConfig.SSH, vm.IPAddress, vm.serverConfig.KubernetesPKISourceDir, "."); err != nil {
			glog.Errorf("scp failed: %v", err)
		} else if _, err = utils.Sudo(vm.serverConfig.SSH, vm.IPAddress, vm.VSphereConfig.Timeout, fmt.Sprintf("mkdir -p %s", vm.serverConfig.KubernetesPKIDestDir)); err != nil {
			glog.Errorf("mkdir failed: %v", err)
		} else if _, err = utils.Sudo(vm.serverConfig.SSH, vm.IPAddress, vm.VSphereConfig.Timeout, fmt.Sprintf("cp -r %s/* %s", filepath.Base(vm.serverConfig.KubernetesPKISourceDir), vm.serverConfig.KubernetesPKIDestDir)); err != nil {
			glog.Errorf("mv failed: %v", err)
		} else if _, err = utils.Sudo(vm.serverConfig.SSH, vm.IPAddress, vm.VSphereConfig.Timeout, fmt.Sprintf("chown -R root:root %s", vm.serverConfig.KubernetesPKIDestDir)); err != nil {
			glog.Errorf("chown failed: %v", err)
		}
	}

	return err
}

func (vm *AutoScalerServerNode) waitForSshReady() error {
	return utils.PollImmediate(time.Second, time.Duration(vm.serverConfig.SSH.WaitSshReadyInSeconds)*time.Second, func() (done bool, err error) {
		if _, err := utils.Sudo(vm.serverConfig.SSH, vm.IPAddress, vm.VSphereConfig.Timeout, "ls"); err != nil {
			if strings.HasSuffix(err.Error(), "connection refused") {
				glog.Warnf("Wait ssh ready for node: %s, address: %s.", vm.NodeName, vm.IPAddress)
				return false, nil
			}

			return false, fmt.Errorf("unable to ssh: %s, reason:%v", vm.NodeName, err)
		}

		return true, nil
	})
}

func (vm *AutoScalerServerNode) executeCommands(args []string, restartKubelet bool, c types.ClientGenerator) error {
	glog.Infof(joinClusterInfo, vm.NodeName, vm.NodeGroupID)

	command := fmt.Sprintf("sh -c \"%s\"", strings.Join(args, " && "))

	if out, err := utils.Sudo(vm.serverConfig.SSH, vm.IPAddress, vm.VSphereConfig.Timeout, command); err != nil {
		return fmt.Errorf(unableToExecuteCmdError, command, out, err)
	} else {

		if restartKubelet {
			// To be sure, with kubeadm 1.26.1, the kubelet is not correctly restarted
			time.Sleep(5 * time.Second)
		}

		return utils.PollImmediate(5*time.Second, time.Duration(vm.serverConfig.SSH.WaitSshReadyInSeconds)*time.Second, func() (done bool, err error) {
			if node, err := c.GetNode(vm.NodeName); err == nil && node != nil {
				return true, nil
			}

			if restartKubelet {
				glog.Infof("Restart kubelet for node:%s for nodegroup: %s", vm.NodeName, vm.NodeGroupID)

				if out, err := utils.Sudo(vm.serverConfig.SSH, vm.IPAddress, vm.VSphereConfig.Timeout, "systemctl restart kubelet"); err != nil {
					return false, fmt.Errorf("unable to restart kubelet, output: %s, reason:%v", out, err)
				}
			}

			return false, nil
		})
	}
}

func (vm *AutoScalerServerNode) kubeAdmJoin(c types.ClientGenerator) error {
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

	command := []string{
		strings.Join(args, " "),
	}

	return vm.executeCommands(command, true, c)
}

func (vm *AutoScalerServerNode) externalAgentJoin(c types.ClientGenerator) error {
	var result error

	kubeAdm := vm.serverConfig.KubeAdm
	external := vm.serverConfig.External
	config := map[string]interface{}{
		"kubelet-arg": []string{
			"cloud-provider=external",
			"fail-swap-on=false",
			fmt.Sprintf("provider-id=%s", vm.generateProviderID()),
			fmt.Sprintf("max-pods=%d", vm.serverConfig.MaxPods),
		},
		"node-name": vm.NodeName,
		"server":    fmt.Sprintf("https://%s", kubeAdm.Address),
		"token":     kubeAdm.Token,
	}

	if external.ExtraConfig != nil {
		for k, v := range external.ExtraConfig {
			config[k] = v
		}
	}

	if vm.ControlPlaneNode {
		if vm.serverConfig.UseControllerManager != nil && *vm.serverConfig.UseControllerManager {
			config["disable-cloud-controller"] = true
		}

		if vm.serverConfig.UseExternalEtdc != nil && *vm.serverConfig.UseExternalEtdc {
			config["datastore-endpoint"] = vm.serverConfig.K3S.DatastoreEndpoint
			config["datastore-cafile"] = fmt.Sprintf("%s/ca.pem", vm.serverConfig.ExtDestinationEtcdSslDir)
			config["datastore-certfile"] = fmt.Sprintf("%s/etcd.pem", vm.serverConfig.ExtDestinationEtcdSslDir)
			config["datastore-keyfile"] = fmt.Sprintf("%s/etcd-key.pem", vm.serverConfig.ExtDestinationEtcdSslDir)
		}
	}

	if f, err := os.CreateTemp(os.TempDir(), "config.*.yaml"); err != nil {
		result = fmt.Errorf("unable to create %s, reason: %v", external.ConfigPath, err)
	} else {
		defer os.Remove(f.Name())

		if _, err = f.WriteString(utils.ToYAML(config)); err != nil {
			f.Close()
			result = fmt.Errorf("unable to write file: %s, reason: %v", f.Name(), err)
		} else {
			f.Close()

			if _, err = f.WriteString(utils.ToYAML(config)); err != nil {
				f.Close()
				result = fmt.Errorf("unable to write file: %s, reason: %v", f.Name(), err)
			} else {
				f.Close()

				const tmpDstFile = "/tmp/config.yaml"

				if err = utils.Scp(vm.serverConfig.SSH, vm.IPAddress, f.Name(), tmpDstFile); err != nil {
					result = fmt.Errorf("unable to transfer file: %s to %s, reason: %v", f.Name(), tmpDstFile, err)
				} else {
					args := []string{
						fmt.Sprintf("mkdir -p %s", path.Dir(external.ConfigPath)),
						fmt.Sprintf("cp %s %s", tmpDstFile, external.ConfigPath),
						external.JoinCommand,
					}

					result = vm.executeCommands(args, false, c)
				}
			}
		}
	}

	return result
}

func (vm *AutoScalerServerNode) rke2AgentJoin(c types.ClientGenerator) error {
	var result error

	kubeAdm := vm.serverConfig.KubeAdm
	k3s := vm.serverConfig.K3S
	service := "rke2-server"
	config := map[string]interface{}{
		"kubelet-arg": []string{
			"cloud-provider=external",
			"fail-swap-on=false",
			fmt.Sprintf("provider-id=%s", vm.generateProviderID()),
			fmt.Sprintf("max-pods=%d", vm.serverConfig.MaxPods),
		},
		"node-name": vm.NodeName,
		"server":    fmt.Sprintf("https://%s", kubeAdm.Address),
		"token":     kubeAdm.Token,
	}

	if vm.ControlPlaneNode {
		if vm.serverConfig.UseControllerManager != nil && *vm.serverConfig.UseControllerManager {
			config["disable-cloud-controller"] = true
			config["cloud-provider-name"] = "external"
		}

		config["disable"] = []string{
			"rke2-ingress-nginx",
			"rke2-metrics-server",
		}

		if vm.serverConfig.UseExternalEtdc != nil && *vm.serverConfig.UseExternalEtdc {
			config["datastore-endpoint"] = k3s.DatastoreEndpoint
			config["datastore-cafile"] = fmt.Sprintf("%s/ca.pem", vm.serverConfig.ExtDestinationEtcdSslDir)
			config["datastore-certfile"] = fmt.Sprintf("%s/etcd.pem", vm.serverConfig.ExtDestinationEtcdSslDir)
			config["datastore-keyfile"] = fmt.Sprintf("%s/etcd-key.pem", vm.serverConfig.ExtDestinationEtcdSslDir)
		}
	}

	// Append extras arguments
	if len(k3s.ExtraCommands) > 0 {
		for _, cmd := range k3s.ExtraCommands {
			args := strings.Split(cmd, "=")
			config[args[0]] = args[1]
		}
	}

	if vm.ControlPlaneNode {
		service = "rke2-server"
	} else {
		service = "rke2-agent"
	}

	const dstFile = "/etc/rancher/rke2/config.yaml"
	const tmpDstFile = "/tmp/config.yaml"

	if f, err := os.CreateTemp(os.TempDir(), "config.*.yaml"); err != nil {
		result = fmt.Errorf("unable to create %s, reason: %v", dstFile, err)
	} else {
		defer os.Remove(f.Name())

		if _, err = f.WriteString(utils.ToYAML(config)); err != nil {
			f.Close()
			result = fmt.Errorf("unable to write file: %s, reason: %v", f.Name(), err)
		} else {
			f.Close()

			if err = utils.Scp(vm.serverConfig.SSH, vm.IPAddress, f.Name(), tmpDstFile); err != nil {
				result = fmt.Errorf("unable to transfer file: %s to %s, reason: %v", f.Name(), tmpDstFile, err)
			} else {
				args := []string{
					fmt.Sprintf("cp %s %s", tmpDstFile, dstFile),
					fmt.Sprintf("systemctl enable %s.service", service),
					fmt.Sprintf("systemctl start %s.service", service),
				}

				result = vm.executeCommands(args, false, c)
			}
		}
	}

	return result
}

func (vm *AutoScalerServerNode) k3sAgentJoin(c types.ClientGenerator) error {
	kubeAdm := vm.serverConfig.KubeAdm
	k3s := vm.serverConfig.K3S
	args := []string{
		fmt.Sprintf("echo K3S_ARGS='--kubelet-arg=provider-id=%s --kubelet-arg=max-pods=%d --node-name=%s --server=https://%s --token=%s' > /etc/systemd/system/k3s.service.env", vm.generateProviderID(), vm.serverConfig.MaxPods, vm.NodeName, kubeAdm.Address, kubeAdm.Token),
	}

	if vm.ControlPlaneNode {
		if vm.serverConfig.UseControllerManager != nil && *vm.serverConfig.UseControllerManager {
			args = append(args, "echo 'K3S_MODE=server' > /etc/default/k3s", "echo K3S_DISABLE_ARGS='--disable-cloud-controller --disable=servicelb --disable=traefik --disable=metrics-server' > /etc/systemd/system/k3s.disabled.env")
		} else {
			args = append(args, "echo 'K3S_MODE=server' > /etc/default/k3s", "echo K3S_DISABLE_ARGS='--disable=servicelb --disable=traefik --disable=metrics-server' > /etc/systemd/system/k3s.disabled.env")
		}

		if vm.serverConfig.UseExternalEtdc != nil && *vm.serverConfig.UseExternalEtdc {
			args = append(args, fmt.Sprintf("echo K3S_SERVER_ARGS='--datastore-endpoint=%s --datastore-cafile=%s/ca.pem --datastore-certfile=%s/etcd.pem --datastore-keyfile=%s/etcd-key.pem' > /etc/systemd/system/k3s.server.env", k3s.DatastoreEndpoint, vm.serverConfig.ExtDestinationEtcdSslDir, vm.serverConfig.ExtDestinationEtcdSslDir, vm.serverConfig.ExtDestinationEtcdSslDir))
		}
	}

	// Append extras arguments
	if len(k3s.ExtraCommands) > 0 {
		args = append(args, k3s.ExtraCommands...)
	}

	args = append(args, "systemctl enable k3s.service", "systemctl start k3s.service")

	return vm.executeCommands(args, false, c)
}

func (vm *AutoScalerServerNode) joinCluster(c types.ClientGenerator) error {
	if vm.serverConfig.Distribution != nil {
		if *vm.serverConfig.Distribution == "k3s" {
			return vm.k3sAgentJoin(c)
		} else if *vm.serverConfig.Distribution == "rke2" {
			return vm.rke2AgentJoin(c)
		} else if *vm.serverConfig.Distribution == "external" {
			return vm.externalAgentJoin(c)
		}
	}

	return vm.kubeAdmJoin(c)
}

func (vm *AutoScalerServerNode) setNodeLabels(c types.ClientGenerator, nodeLabels, systemLabels types.KubernetesLabel) error {
	labels := utils.MergeKubernetesLabel(nodeLabels, systemLabels, vm.ExtraLabels)

	if err := c.LabelNode(vm.NodeName, labels); err != nil {
		return fmt.Errorf(constantes.ErrLabelNodeReturnError, vm.NodeName, err)
	}

	annotations := types.KubernetesLabel{
		constantes.AnnotationNodeGroupName:        vm.NodeGroupID,
		constantes.AnnotationInstanceID:           vm.VMUUID,
		constantes.AnnotationScaleDownDisabled:    strconv.FormatBool(vm.NodeType != AutoScalerServerNodeAutoscaled),
		constantes.AnnotationNodeAutoProvisionned: strconv.FormatBool(vm.NodeType == AutoScalerServerNodeAutoscaled),
		constantes.AnnotationNodeManaged:          strconv.FormatBool(vm.NodeType == AutoScalerServerNodeManaged),
		constantes.AnnotationNodeIndex:            strconv.Itoa(vm.NodeIndex),
	}

	annotations = utils.MergeKubernetesLabel(annotations, vm.ExtraAnnotations)

	if err := c.AnnoteNode(vm.NodeName, annotations); err != nil {
		return fmt.Errorf(constantes.ErrAnnoteNodeReturnError, vm.NodeName, err)
	}

	if vm.ControlPlaneNode && vm.AllowDeployment {
		if err := c.TaintNode(vm.NodeName,
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
			}); err != nil {
			return fmt.Errorf(constantes.ErrTaintNodeReturnError, vm.NodeName, err)
		}
	}

	return nil
}

func (vm *AutoScalerServerNode) launchVM(c types.ClientGenerator, nodeLabels, systemLabels types.KubernetesLabel) error {
	glog.Debugf("AutoScalerNode::launchVM, node:%s", vm.NodeName)

	var err error
	var status AutoScalerServerNodeState
	var hostsystem string

	vsphere := vm.VSphereConfig
	network := vsphere.Network
	userInfo := vm.serverConfig.SSH

	glog.Infof("Launch VM:%s for nodegroup: %s", vm.NodeName, vm.NodeGroupID)

	if vm.State != AutoScalerServerNodeStateNotCreated {
		return fmt.Errorf(constantes.ErrVMAlreadyCreated, vm.NodeName)
	}

	if vsphere.Exists(vm.NodeName) {
		glog.Warnf(constantes.ErrVMAlreadyExists, vm.NodeName)
		return fmt.Errorf(constantes.ErrVMAlreadyExists, vm.NodeName)
	}

	vm.State = AutoScalerServerNodeStateCreating

	if vm.NodeType != AutoScalerServerNodeAutoscaled && vm.NodeType != AutoScalerServerNodeManaged {

		err = fmt.Errorf(constantes.ErrVMNotProvisionnedByMe, vm.NodeName)

	} else if _, err = vsphere.Create(vm.NodeName, userInfo.GetUserName(), userInfo.GetAuthKeys(), vm.serverConfig.CloudInit, network, "", true, vm.Memory, vm.CPU, vm.Disk, vm.NodeIndex); err != nil {

		err = fmt.Errorf(constantes.ErrUnableToLaunchVM, vm.NodeName, err)

	} else if vm.VMUUID, err = vsphere.UUID(vm.NodeName); err != nil {

		err = fmt.Errorf(constantes.ErrStartVMFailed, vm.NodeName, err)

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

	} else if err = vm.waitForSshReady(); err != nil {

		err = fmt.Errorf(constantes.ErrStartVMFailed, vm.NodeName, err)

	} else if err = vm.recopyKubernetesPKIIfNeeded(); err != nil {

		err = fmt.Errorf(constantes.ErrRecopyKubernetesPKIFailed, vm.NodeName, err)

	} else if err = vm.recopyEtcdSslFilesIfNeeded(); err != nil {

		err = fmt.Errorf(constantes.ErrUpdateEtcdSslFailed, vm.NodeName, err)

	} else if err = vm.joinCluster(c); err != nil {

		err = fmt.Errorf(constantes.ErrKubeAdmJoinFailed, vm.NodeName, err)

	} else if err = vm.setProviderID(c); err != nil {

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

		} else if _, err = vsphere.WaitForToolsRunning(vm.NodeName); err != nil {

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
				// Delete kubernetes node only is alive
				if _, err = c.GetNode(vm.NodeName); err == nil {
					if err = c.MarkDrainNode(vm.NodeName); err != nil {
						glog.Errorf(constantes.ErrCordonNodeReturnError, vm.NodeName, err)
					}

					if err = c.DrainNode(vm.NodeName, true, true); err != nil {
						glog.Errorf(constantes.ErrDrainNodeReturnError, vm.NodeName, err)
					}

					if err = c.DeleteNode(vm.NodeName); err != nil {
						glog.Errorf(constantes.ErrDeleteNodeReturnError, vm.NodeName, err)
					}
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

func (vm *AutoScalerServerNode) setProviderID(c types.ClientGenerator) error {
	if vm.serverConfig.UseControllerManager != nil && !*vm.serverConfig.UseControllerManager {
		return c.SetProviderID(vm.NodeName, vm.generateProviderID())
	}

	return nil
}

func (vm *AutoScalerServerNode) generateProviderID() string {
	return fmt.Sprintf("vsphere://%s", vm.VMUUID)
}

func (vm *AutoScalerServerNode) findInstanceUUID() string {
	if vmUUID, err := vm.VSphereConfig.UUID(vm.NodeName); err == nil {
		vm.VMUUID = vmUUID

		return vmUUID
	}

	return ""
}

func (vm *AutoScalerServerNode) setServerConfiguration(config *types.AutoScalerServerConfig) {
	vm.VSphereConfig.Network.UpdateMacAddressTable(vm.NodeIndex)
	vm.serverConfig = config
}

func (vm *AutoScalerServerNode) retrieveNetworkInfos() error {
	return vm.VSphereConfig.RetrieveNetworkInfos(vm.NodeName, vm.NodeIndex)
}
