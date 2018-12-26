package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	apiv1 "k8s.io/api/core/v1"

	"github.com/golang/glog"
	"github.com/vmware/govmomi/vim25/types"
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
	NodeName         string                    `json:"name"`
	NodeIndex        int                       `json:"index"`
	Memory           int                       `json:"memory"`
	CPU              int                       `json:"cpu"`
	Disk             int                       `json:"disk"`
	Addresses        []string                  `json:"addresses"`
	State            AutoScalerServerNodeState `json:"state"`
	AutoProvisionned bool                      `json:"auto"`
}

func (vm *AutoScalerServerNode) getVM() *vsphereInfos {
	vsphere := newVSphereInfos(vm.NodeName)

	return vsphere
}

func (vm *AutoScalerServerNode) prepareKubelet() error {
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
		return fmt.Errorf(errKubeletNotConfigured, vm.NodeName, out, err)
	}

	defer os.Remove(fName)

	if err = scp(vm.Addresses[0], fName, fName); err != nil {
		return fmt.Errorf(errKubeletNotConfigured, vm.NodeName, out, err)
	}

	if out, err = sudo(vm.Addresses[0], fmt.Sprintf("bash %s", fName)); err != nil {
		return fmt.Errorf(errKubeletNotConfigured, vm.NodeName, out, err)
	}

	return nil
}

func (vm *AutoScalerServerNode) waitReady(kubeconfig string) error {
	glog.V(5).Infof("AutoScalerNode::waitReady, node:%s", vm.NodeName)

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

		if out, err = pipe(arg...); err != nil {
			return err
		}

		var nodeInfo apiv1.Node

		if err = json.Unmarshal([]byte(out), &nodeInfo); err != nil {
			return fmt.Errorf(errUnmarshallingError, vm.NodeName, err)
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

	return fmt.Errorf(errNodeIsNotReady, vm.NodeName)
}

func (vm *AutoScalerServerNode) kubeAdmJoin(extras *nodeCreationExtra) error {
	args := []string{
		"kubeadm",
		"join",
		extras.kubeHost,
		"--token",
		extras.kubeToken,
		"--discovery-token-ca-cert-hash",
		extras.kubeCACert,
	}

	// Append extras arguments
	if len(extras.kubeExtraArgs) > 0 {
		args = append(args, extras.kubeExtraArgs...)
	}

	if _, err := sudo(vm.Addresses[0], strings.Join(args, " ")); err != nil {
		return fmt.Errorf(errKubeAdmJoinFailed, vm.NodeName, err)
	}

	return nil
}

func (vm *AutoScalerServerNode) setNodeLabels(extras *nodeCreationExtra) error {
	if len(extras.nodeLabels)+len(extras.systemLabels) > 0 {

		args := []string{
			"kubectl",
			"label",
			"nodes",
			vm.NodeName,
		}

		// Append extras arguments
		for k, v := range extras.nodeLabels {
			args = append(args, fmt.Sprintf("%s=%s", k, v))
		}

		for k, v := range extras.systemLabels {
			args = append(args, fmt.Sprintf("%s=%s", k, v))
		}

		args = append(args, "--kubeconfig")
		args = append(args, extras.kubeConfig)

		if err := shell(args...); err != nil {
			return fmt.Errorf(errKubeCtlIgnoredError, vm.NodeName, err)
		}
	}

	args := []string{
		"kubectl",
		"annotate",
		"node",
		vm.NodeName,
		fmt.Sprintf("%s=%s", nodeLabelGroupName, extras.nodegroupID),
		fmt.Sprintf("%s=%s", annotationNodeAutoProvisionned, strconv.FormatBool(vm.AutoProvisionned)),
		fmt.Sprintf("%s=%d", annotationNodeIndex, vm.NodeIndex),
		"--overwrite",
		"--kubeconfig",
		extras.kubeConfig,
	}

	if err := shell(args...); err != nil {
		return fmt.Errorf(errKubeCtlIgnoredError, vm.NodeName, err)
	}

	return nil
}

func (vm *AutoScalerServerNode) mountPoints(extras *nodeCreationExtra) error {
	/* 	if extras.mountPoints != nil && len(extras.mountPoints) > 0 {
	   		for hostPath, guestPath := range extras.mountPoints {
	   			if err = shell("AutoScaler", "mount", hostPath, fmt.Sprintf("%s:%s", vm.NodeName, guestPath)); err != nil {
	   				glog.Warningf(errUnableToMountPath, hostPath, guestPath, vm.NodeName, err)
	   			}
	   		}
	   	}
	*/
	return nil
}

func (vm *AutoScalerServerNode) launchVM(extras *nodeCreationExtra) error {
	glog.V(5).Infof("AutoScalerNode::launchVM, node:%s", vm.NodeName)

	var err error
	var status AutoScalerServerNodeState

	glog.Infof("Launch VM:%s for nodegroup: %s", vm.NodeName, extras.nodegroupID)

	if vm.AutoProvisionned == false {
		err = fmt.Errorf(errVMNotProvisionnedByMe, vm.NodeName)
	} else if vm.State != AutoScalerServerNodeStateNotCreated {
		err = fmt.Errorf(errVMAlreadyCreated, vm.NodeName)
	} else {
		vsphere := vm.getVM()

		// Launch the VM and wait until finish launched
		if err = vsphere.create(extras.image, vm.Memory, vm.CPU, vm.Disk); err != nil {
			err = fmt.Errorf(errUnableToLaunchVM, vm.NodeName, err)
		} else {
			// Add mount point
			vm.mountPoints(extras)

			if status, err = vm.statusVM(); err == nil {
				if status == AutoScalerServerNodeStateRunning {
					// If the VM is running call kubeadm join
					if extras.vmprovision {
						if err = vm.prepareKubelet(); err == nil {
							if err = vm.kubeAdmJoin(extras); err == nil {
								if err = vm.waitReady(extras.kubeConfig); err == nil {
									err = vm.setNodeLabels(extras)
								}
							}
						}
					}
				} else {
					err = fmt.Errorf(errKubeAdmJoinNotRunning, vm.NodeName)
				}
			}
		}
	}

	if err == nil {
		glog.Infof("Launched VM:%s for nodegroup: %s", vm.NodeName, extras.nodegroupID)
	} else {
		glog.Errorf("Unable to launch VM:%s for nodegroup: %s. Reason: %v", vm.NodeName, extras.nodegroupID, err.Error())
	}

	return err
}

func (vm *AutoScalerServerNode) startVM(kubeconfig string) error {
	glog.V(5).Infof("AutoScalerNode::startVM, node:%s", vm.NodeName)

	var err error
	var state AutoScalerServerNodeState

	glog.Infof("Start VM:%s", vm.NodeName)

	if vm.AutoProvisionned == false {
		err = fmt.Errorf(errVMNotProvisionnedByMe, vm.NodeName)
	} else {
		state, err = vm.statusVM()

		if err == nil {
			if state == AutoScalerServerNodeStateStopped {
				vsphere := vm.getVM()

				if err = vsphere.powerOn(); err != nil {
					args := []string{
						"kubectl",
						"uncordon",
						vm.NodeName,
						"--kubeconfig",
						kubeconfig,
					}

					if err = shell(args...); err != nil {
						glog.Errorf(errKubeCtlIgnoredError, vm.NodeName, err)

						err = nil
					}

					vm.State = AutoScalerServerNodeStateRunning
				} else {
					err = fmt.Errorf(errStartVMFailed, vm.NodeName, err)
				}
			} else if state != AutoScalerServerNodeStateRunning {
				err = fmt.Errorf(errStartVMFailed, vm.NodeName, fmt.Sprintf("Unexpected state: %d", state))
			}
		}
	}

	if err == nil {
		glog.Infof("Started VM:%s", vm.NodeName)
	} else {
		glog.Errorf("Unable to start VM:%s. Reason: %v", vm.NodeName, err)
	}

	return err
}

func (vm *AutoScalerServerNode) stopVM(kubeconfig string) error {
	glog.V(5).Infof("AutoScalerNode::stopVM, node:%s", vm.NodeName)

	var err error
	var state AutoScalerServerNodeState

	glog.Infof("Stop VM:%s", vm.NodeName)

	if vm.AutoProvisionned == false {
		err = fmt.Errorf(errVMNotProvisionnedByMe, vm.NodeName)
	} else {
		state, err = vm.statusVM()

		if err == nil {
			if state == AutoScalerServerNodeStateRunning {
				vsphere := vm.getVM()
				args := []string{
					"kubectl",
					"cordon",
					vm.NodeName,
					"--kubeconfig",
					kubeconfig,
				}

				if err = shell(args...); err != nil {
					glog.Errorf(errKubeCtlIgnoredError, vm.NodeName, err)
				}

				if err = vsphere.powerOff(); err == nil {
					vm.State = AutoScalerServerNodeStateStopped
				} else {
					err = fmt.Errorf(errStopVMFailed, vm.NodeName, err)
				}
			} else if state != AutoScalerServerNodeStateStopped {
				err = fmt.Errorf(errStopVMFailed, vm.NodeName, fmt.Sprintf("Unexpected state: %d", state))
			}
		}
	}

	if err == nil {
		glog.Infof("Stopped VM:%s", vm.NodeName)
	} else {
		glog.Errorf("Could not stop VM:%s. Reason: %s", vm.NodeName, err)
	}

	return err
}

func (vm *AutoScalerServerNode) deleteVM(kubeconfig string) error {
	glog.V(5).Infof("AutoScalerNode::deleteVM, node:%s", vm.NodeName)

	var err error
	var state AutoScalerServerNodeState

	vsphere := vm.getVM()

	if vm.AutoProvisionned == false {
		err = fmt.Errorf(errVMNotProvisionnedByMe, vm.NodeName)
	} else {
		state, err = vm.statusVM()

		if err == nil {
			if state == AutoScalerServerNodeStateRunning {
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

				if err = shell(args...); err != nil {
					glog.Errorf(errKubeCtlIgnoredError, vm.NodeName, err)
				}

				args = []string{
					"kubectl",
					"delete",
					"node",
					vm.NodeName,
					"--kubeconfig",
					kubeconfig,
				}

				if err = shell(args...); err != nil {
					glog.Errorf(errKubeCtlIgnoredError, vm.NodeName, err)
				}

				if err = vsphere.powerOff(); err == nil {
					vm.State = AutoScalerServerNodeStateStopped

					if err = vsphere.delete(); err == nil {
						vm.State = AutoScalerServerNodeStateDeleted
					} else {
						err = fmt.Errorf(errDeleteVMFailed, vm.NodeName, err)
					}
				} else {
					err = fmt.Errorf(errStopVMFailed, vm.NodeName, err)
				}
			} else if err = vsphere.delete(); err == nil {
				vm.State = AutoScalerServerNodeStateDeleted
			} else {
				err = fmt.Errorf(errDeleteVMFailed, vm.NodeName, err)
			}
		}
	}

	if err == nil {
		glog.Infof("Deleted VM:%s", vm.NodeName)
	} else {
		glog.Errorf("Could not delete VM:%s. Reason: %s", vm.NodeName, err)
	}

	return err
}

func (vm *AutoScalerServerNode) statusVM() (AutoScalerServerNodeState, error) {
	glog.V(5).Infof("AutoScalerNode::statusVM, node:%s", vm.NodeName)

	// Get VM infos
	var vmInfos *infoResult
	var err error

	vsphere := vm.getVM()

	if vmInfos, err = vsphere.status(); err != nil {
		glog.Errorf(errGetVMInfoFailed, vm.NodeName, err)
		return AutoScalerServerNodeStateUndefined, err
	}

	if vmInfos != nil {
		vm.Addresses = []string{
			vmInfos.VirtualMachines.Guest.IpAddress,
		}

		switch vmInfos.VirtualMachines.Runtime.PowerState {
		case types.VirtualMachinePowerStatePoweredOff:
			vm.State = AutoScalerServerNodeStateStopped
		case types.VirtualMachinePowerStatePoweredOn:
			vm.State = AutoScalerServerNodeStateRunning
		case types.VirtualMachinePowerStateSuspended:
			vm.State = AutoScalerServerNodeStateStopped
		default:
			vm.State = AutoScalerServerNodeStateUndefined
		}

		return vm.State, nil
	}

	return AutoScalerServerNodeStateUndefined, fmt.Errorf(errAutoScalerInfoNotFound, vm.NodeName)
}
