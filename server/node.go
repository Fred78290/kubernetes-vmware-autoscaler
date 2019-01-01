package server

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	apiv1 "k8s.io/api/core/v1"

	"github.com/Fred78290/kubernetes-vmware-autoscaler/constantes"
	"github.com/Fred78290/kubernetes-vmware-autoscaler/types"
	"github.com/Fred78290/kubernetes-vmware-autoscaler/utils"
	"github.com/Fred78290/kubernetes-vmware-autoscaler/vsphere"
	"github.com/golang/glog"
)

// AutoScalerServerNodeState VM state
type AutoScalerServerNodeState int32

const (
	// AutoScalerServerNodeStateNotCreated not created state
	AutoScalerServerNodeStateNotCreated AutoScalerServerNodeState = 0

	// AutoScalerServerNodeStateRunning running state
	AutoScalerServerNodeStateRunning AutoScalerServerNodeState = 1

	// AutoScalerServerNodeStateStopped stopped state
	AutoScalerServerNodeStateStopped AutoScalerServerNodeState = 2

	// AutoScalerServerNodeStateDeleted deleted state
	AutoScalerServerNodeStateDeleted AutoScalerServerNodeState = 3

	// AutoScalerServerNodeStateUndefined undefined state
	AutoScalerServerNodeStateUndefined AutoScalerServerNodeState = 4
)

// AutoScalerServerNode Describe a AutoScaler VM
type AutoScalerServerNode struct {
	ProviderID       string                    `json:"providerID"`
	NodeGroupID      string                    `json:"group"`
	NodeName         string                    `json:"name"`
	NodeIndex        int                       `json:"index"`
	Memory           int                       `json:"memory"`
	CPU              int                       `json:"cpu"`
	Disk             int                       `json:"disk"`
	Addresses        []string                  `json:"addresses"`
	State            AutoScalerServerNodeState `json:"state"`
	AutoProvisionned bool                      `json:"auto"`
	configuration    *types.AutoScalerServerConfig
}

func (vm *AutoScalerServerNode) prepareKubelet() (string, error) {
	var out string
	var err error
	var fName = fmt.Sprintf("/tmp/set-kubelet-default-%s.sh", vm.NodeName)

	kubeletDefault := []string{
		"#!/bin/bash",
		". /etc/default/kubelet",
		fmt.Sprintf("echo \"KUBELET_EXTRA_ARGS=\\\"$KUBELET_EXTRA_ARGS --provider-id=%s\\\"\" > /etc/default/kubelet", vm.ProviderID),
		"systemctl restart kubelet",
	}

	if err = ioutil.WriteFile(fName, []byte(strings.Join(kubeletDefault, "\n")), 0755); err != nil {
		return out, err
	}

	defer os.Remove(fName)

	if err = utils.Scp(vm.Addresses[0], fName, fName); err != nil {
		return out, err
	}

	if out, err = utils.Sudo(vm.configuration.SSH, vm.Addresses[0], fmt.Sprintf("bash %s", fName)); err != nil {
		return out, err
	}

	return "", nil
}

func (vm *AutoScalerServerNode) waitReady() error {
	glog.V(5).Infof("AutoScalerNode::waitReady, node:%s", vm.NodeName)

	kubeconfig := vm.configuration.KubeCtlConfig

	// Max 60s
	for index := 0; index < 12; index++ {
		var out string
		var err error
		var arg = []string{
			"kubectl",
			"get",
			"nodes",
			vm.NodeName,
			"--output",
			"json",
			"--kubeconfig",
			kubeconfig,
		}

		if out, err = utils.Pipe(arg...); err != nil {
			return err
		}

		var nodeInfo apiv1.Node

		if err = json.Unmarshal([]byte(out), &nodeInfo); err != nil {
			return fmt.Errorf(constantes.ErrUnmarshallingError, vm.NodeName, err)
		}

		for _, status := range nodeInfo.Status.Conditions {
			if status.Type == "Ready" {
				if b, e := strconv.ParseBool(string(status.Status)); e == nil {
					if b {
						glog.Infof("The kubernetes node %s is Ready", vm.NodeName)
						return nil
					}
				}
			}
		}

		glog.Infof("The kubernetes node:%s is not ready", vm.NodeName)

		time.Sleep(5 * time.Second)
	}

	return fmt.Errorf(constantes.ErrNodeIsNotReady, vm.NodeName)
}

func (vm *AutoScalerServerNode) kubeAdmJoin() error {
	kubeAdm := vm.configuration.KubeAdm

	args := []string{
		"kubeadm",
		"join",
		kubeAdm.Address,
		"--token",
		kubeAdm.Token,
		"--discovery-token-ca-cert-hash",
		kubeAdm.CACert,
	}

	// Append extras arguments
	if len(kubeAdm.ExtraArguments) > 0 {
		args = append(args, kubeAdm.ExtraArguments...)
	}

	if _, err := utils.Sudo(vm.configuration.SSH, vm.Addresses[0], strings.Join(args, " ")); err != nil {
		return err
	}

	return nil
}

func (vm *AutoScalerServerNode) setNodeLabels(nodeLabels, systemLabels KubernetesLabel) error {
	if len(nodeLabels)+len(systemLabels) > 0 {

		args := []string{
			"kubectl",
			"label",
			"nodes",
			vm.NodeName,
		}

		// Append extras arguments
		for k, v := range nodeLabels {
			args = append(args, fmt.Sprintf("%s=%s", k, v))
		}

		for k, v := range systemLabels {
			args = append(args, fmt.Sprintf("%s=%s", k, v))
		}

		args = append(args, "--kubeconfig")
		args = append(args, vm.configuration.KubeCtlConfig)

		if out, err := utils.Pipe(args...); err != nil {
			return fmt.Errorf(constantes.ErrKubeCtlReturnError, vm.NodeName, out, err)
		}
	}

	args := []string{
		"kubectl",
		"annotate",
		"node",
		vm.NodeName,
		fmt.Sprintf("%s=%s", constantes.NodeLabelGroupName, vm.NodeGroupID),
		fmt.Sprintf("%s=%s", constantes.AnnotationNodeAutoProvisionned, strconv.FormatBool(vm.AutoProvisionned)),
		fmt.Sprintf("%s=%d", constantes.AnnotationNodeIndex, vm.NodeIndex),
		"--overwrite",
		"--kubeconfig",
		vm.configuration.KubeCtlConfig,
	}

	if out, err := utils.Pipe(args...); err != nil {
		return fmt.Errorf(constantes.ErrKubeCtlReturnError, vm.NodeName, out, err)
	}

	return nil
}

var phDefaultRsyncFlags = []string{
	"--verbose",
	"--archive",
	"-z",
	"--copy-links",
	"--no-owner",
	"--no-group",
	"--delete",
}

func (vm *AutoScalerServerNode) syncFolders() (string, error) {

	syncFolders := vm.configuration.SyncFolders

	if syncFolders != nil && len(syncFolders.Folders) > 0 {
		for _, folder := range syncFolders.Folders {
			var rsync = []string{
				"rsync",
			}

			tempFile, _ := ioutil.TempFile(os.TempDir(), "vmware-rsync")

			defer tempFile.Close()

			if len(syncFolders.RsyncOptions) == 0 {
				rsync = append(rsync, phDefaultRsyncFlags...)
			} else {
				rsync = append(rsync, syncFolders.RsyncOptions...)
			}

			sshOptions := []string{
				"--rsync-path",
				"sudo rsync",
				"-e",
				fmt.Sprintf("ssh -p 22 -o LogLevel=FATAL -o ControlMaster=auto -o ControlPath=%s -o ControlPersist=10m  -o IdentitiesOnly=yes -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i '%s'", tempFile.Name(), syncFolders.RsyncSSHKey),
			}

			excludes := make([]string, 0, len(folder.Excludes)*2)

			for _, exclude := range folder.Excludes {
				excludes = append(excludes, "--exclude", exclude)
			}

			rsync = append(rsync, sshOptions...)
			rsync = append(rsync, excludes...)
			rsync = append(rsync, folder.Source, fmt.Sprintf("%s@%s:%s", syncFolders.RsyncUser, vm.Addresses[0], folder.Destination))

			if out, err := utils.Pipe(rsync...); err != nil {
				return out, err
			}
		}
	}
	//["/usr/bin/rsync", "--verbose", "--archive", "--delete", "-z", "--copy-links", "--no-owner", "--no-group", "--rsync-path", "sudo rsync", "-e",
	// "ssh -p 22 -o LogLevel=FATAL  -o ControlMaster=auto -o ControlPath=/tmp/vagrant-rsync-20181227-31508-1sjw4bm -o ControlPersist=10m  -o IdentitiesOnly=yes -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i '/home/fboltz/.ssh/id_rsa'",
	// "--exclude", ".vagrant/", "/home/fboltz/Projects/vagrant-multipass/", "vagrant@10.196.85.125:/vagrant"]
	return "", nil
}

func (vm *AutoScalerServerNode) launchVM(nodeLabels, systemLabels KubernetesLabel) error {
	glog.V(5).Infof("AutoScalerNode::launchVM, node:%s", vm.NodeName)

	var err error
	var status *vsphere.Status
	var output string

	vsphere := vm.configuration.VSphere
	network := vsphere.Network
	userInfo := vm.configuration.SSH

	glog.Infof("Launch VM:%s for nodegroup: %s", vm.NodeName, vm.NodeGroupID)

	if vm.AutoProvisionned == false {

		err = fmt.Errorf(constantes.ErrVMNotProvisionnedByMe, vm.NodeName)

	} else if vm.State != AutoScalerServerNodeStateNotCreated {

		err = fmt.Errorf(constantes.ErrVMAlreadyCreated, vm.NodeName)

	} else if _, err = vsphere.Create(vm.NodeName, userInfo.UserName, userInfo.AuthKeys, vm.configuration.CloudInit, network, "", vm.Memory, vm.CPU, vm.Disk); err != nil {

		err = fmt.Errorf(constantes.ErrUnableToLaunchVM, vm.NodeName, err)

	} else if err = vsphere.PowerOn(vm.NodeName); err != nil {

		err = fmt.Errorf(constantes.ErrStartVMFailed, vm.NodeName, err)

	} else if _, err = vsphere.WaitForIP(vm.NodeName); err != nil {

		err = fmt.Errorf(constantes.ErrStartVMFailed, vm.NodeName, err)

	} else if status, err = vsphere.Status(vm.NodeName); err != nil {

		err = fmt.Errorf(constantes.ErrGetVMInfoFailed, vm.NodeName, err)

	} else if status.Powered == false {

		err = fmt.Errorf(constantes.ErrStartVMFailed, vm.NodeName, err)

	} else if output, err = vm.syncFolders(); err != nil {

		err = fmt.Errorf(constantes.ErrRsyncError, vm.NodeName, output, err)

	} else if output, err = vm.prepareKubelet(); err != nil {

		err = fmt.Errorf(constantes.ErrKubeletNotConfigured, vm.NodeName, output, err)

	} else if err = vm.kubeAdmJoin(); err != nil {

		err = fmt.Errorf(constantes.ErrKubeAdmJoinFailed, vm.NodeName, err)

	} else if err = vm.waitReady(); err != nil {

		err = fmt.Errorf(constantes.ErrNodeIsNotReady, vm.NodeName)

	} else {
		err = vm.setNodeLabels(nodeLabels, systemLabels)
	}

	if err == nil {
		glog.Infof("Launched VM:%s for nodegroup: %s", vm.NodeName, vm.NodeGroupID)
	} else {
		glog.Errorf("Unable to launch VM:%s for nodegroup: %s. Reason: %v", vm.NodeName, vm.NodeGroupID, err.Error())
	}

	return err
}

func (vm *AutoScalerServerNode) startVM() error {
	glog.V(5).Infof("AutoScalerNode::startVM, node:%s", vm.NodeName)

	var err error
	var state AutoScalerServerNodeState

	glog.Infof("Start VM:%s", vm.NodeName)

	kubeconfig := vm.configuration.KubeCtlConfig

	if vm.AutoProvisionned == false {

		err = fmt.Errorf(constantes.ErrVMNotProvisionnedByMe, vm.NodeName)

	} else if state, err = vm.statusVM(); err != nil {

		err = fmt.Errorf(constantes.ErrStartVMFailed, vm.NodeName, err)

	} else if state == AutoScalerServerNodeStateStopped {

		if err = vm.configuration.VSphere.PowerOn(vm.NodeName); err != nil {

			err = fmt.Errorf(constantes.ErrStartVMFailed, vm.NodeName, err)

		} else if _, err = vm.configuration.VSphere.WaitForIP(vm.NodeName); err != nil {

			err = fmt.Errorf(constantes.ErrStartVMFailed, vm.NodeName, err)

		} else if state, err = vm.statusVM(); err != nil {

			err = fmt.Errorf(constantes.ErrStartVMFailed, vm.NodeName, err)

		} else if state != AutoScalerServerNodeStateRunning {

			err = fmt.Errorf(constantes.ErrStartVMFailed, vm.NodeName, err)

		} else {
			args := []string{
				"kubectl",
				"uncordon",
				vm.NodeName,
				"--kubeconfig",
				kubeconfig,
			}

			if err = utils.Shell(args...); err != nil {
				glog.Errorf(constantes.ErrKubeCtlIgnoredError, vm.NodeName, err)

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

func (vm *AutoScalerServerNode) stopVM() error {
	glog.V(5).Infof("AutoScalerNode::stopVM, node:%s", vm.NodeName)

	var err error
	var state AutoScalerServerNodeState

	glog.Infof("Stop VM:%s", vm.NodeName)

	kubeconfig := vm.configuration.KubeCtlConfig

	if vm.AutoProvisionned == false {

		err = fmt.Errorf(constantes.ErrVMNotProvisionnedByMe, vm.NodeName)

	} else if state, err = vm.statusVM(); err != nil {

		err = fmt.Errorf(constantes.ErrStopVMFailed, vm.NodeName, err)

	} else if state == AutoScalerServerNodeStateRunning {

		args := []string{
			"kubectl",
			"cordon",
			vm.NodeName,
			"--kubeconfig",
			kubeconfig,
		}

		if err = utils.Shell(args...); err != nil {
			glog.Errorf(constantes.ErrKubeCtlIgnoredError, vm.NodeName, err)
		}

		if err = vm.configuration.VSphere.PowerOff(vm.NodeName); err == nil {
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

func (vm *AutoScalerServerNode) deleteVM() error {
	glog.V(5).Infof("AutoScalerNode::deleteVM, node:%s", vm.NodeName)

	var err error
	var status *vsphere.Status

	if vm.AutoProvisionned == false {
		err = fmt.Errorf(constantes.ErrVMNotProvisionnedByMe, vm.NodeName)
	} else {
		kubeconfig := vm.configuration.KubeCtlConfig
		vsphere := vm.configuration.VSphere

		if status, err = vsphere.Status(vm.NodeName); err == nil {
			if status.Powered {
				args := []string{
					"kubectl",
					"drain",
					vm.NodeName,
					"--delete-local-data",
					"--force",
					"--ignore-daemonsets",
					"--kubeconfig",
					kubeconfig,
				}

				if err = utils.Shell(args...); err != nil {
					glog.Errorf(constantes.ErrKubeCtlIgnoredError, vm.NodeName, err)
				}

				args = []string{
					"kubectl",
					"delete",
					"node",
					vm.NodeName,
					"--kubeconfig",
					kubeconfig,
				}

				if err = utils.Shell(args...); err != nil {
					glog.Errorf(constantes.ErrKubeCtlIgnoredError, vm.NodeName, err)
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
	glog.V(5).Infof("AutoScalerNode::statusVM, node:%s", vm.NodeName)

	// Get VM infos
	var status *vsphere.Status
	var err error

	if status, err = vm.configuration.VSphere.Status(vm.NodeName); err != nil {
		glog.Errorf(constantes.ErrGetVMInfoFailed, vm.NodeName, err)
		return AutoScalerServerNodeStateUndefined, err
	}

	if status != nil {
		vm.Addresses = []string{
			status.Address,
		}

		if status.Powered {
			vm.State = AutoScalerServerNodeStateRunning
		} else {
			vm.State = AutoScalerServerNodeStateStopped
		}

		return vm.State, nil
	}

	return AutoScalerServerNodeStateUndefined, fmt.Errorf(constantes.ErrAutoScalerInfoNotFound, vm.NodeName)
}

func (vm *AutoScalerServerNode) setConfiguration(config *types.AutoScalerServerConfig) {
	vm.configuration = config
}
