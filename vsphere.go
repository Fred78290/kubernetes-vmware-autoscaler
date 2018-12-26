package main

import (
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// VSphereInfos VSphereInfos
type vsphereInfos struct {
	vmName string
}

type infoResult struct {
	VirtualMachines mo.VirtualMachine
	objects         *object.VirtualMachine
	entities        map[types.ManagedObjectReference]string
}

func newVSphereInfos(vm string) *vsphereInfos {
	return &vsphereInfos{
		vmName: vm,
	}
}

func (vm *vsphereInfos) create(templateName string, memory int, cpus int, disk int) error {
	return nil
}

func (vm *vsphereInfos) delete() error {
	return nil
}

func (vm *vsphereInfos) powerOn() error {
	return nil
}

func (vm *vsphereInfos) powerOff() error {
	return nil
}

func (vm *vsphereInfos) status() (*infoResult, error) {
	return nil, nil
}
