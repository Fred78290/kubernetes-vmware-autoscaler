package vsphere

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/Fred78290/kubernetes-vmware-autoscaler/constantes"
	"github.com/golang/glog"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25"
	yaml "gopkg.in/yaml.v2"

	"github.com/vmware/govmomi/vim25/types"
)

// VirtualMachine virtual machine wrapper
type VirtualMachine struct {
	Ref       types.ManagedObjectReference
	Name      string
	Datastore *Datastore
}

// GuestInfos the guest infos
// Must not start with `guestinfo.`
type GuestInfos map[string]string

type extraConfig []types.BaseOptionValue

// NetworkAdapter wrapper
type NetworkAdapter struct {
	DHCP4 bool              `json:"dhcp4"`
	Name  string            `json:"set-name,omitempty"`
	Match map[string]string `json:"match,omitempty"`
}

// NetworkDeclare wrapper
type NetworkDeclare struct {
	Version   int                       `json:"version"`
	Ethernets map[string]NetworkAdapter `json:"ethernets"`
}

// NetworkConfig wrapper
type NetworkConfig struct {
	Network NetworkDeclare `json:"network"`
}

func encodeObject(name string, object interface{}) (string, error) {
	var result string
	out, err := yaml.Marshal(object)

	if err == nil {
		var stdout bytes.Buffer
		var zw = gzip.NewWriter(&stdout)

		zw.Name = name
		zw.ModTime = time.Now()

		if _, err = zw.Write(out); err == nil {
			if err = zw.Close(); err == nil {
				result = base64.StdEncoding.EncodeToString(stdout.Bytes())
			}
		}
	}

	return result, err
}

func buildVendorData(userName, authKey string) interface{} {
	tz, _ := time.Now().Zone()

	return map[string]interface{}{
		"package_update":  true,
		"package_upgrade": true,
		"timezone":        tz,
		"users": []string{
			"default",
		},
		"ssh_authorized_keys": []string{
			authKey,
		},
		"system_info": map[string]interface{}{
			"default_user": map[string]string{
				"name": userName,
			},
		},
	}
}

func buildNetworkConfig(network *Network) NetworkConfig {
	var match map[string]string

	if len(network.Address) > 0 {
		match = map[string]string{
			"macaddress": network.Address,
		}
	}

	net := NetworkConfig{
		Network: NetworkDeclare{
			Version: 2,
			Ethernets: map[string]NetworkAdapter{
				network.NicName: NetworkAdapter{
					DHCP4: true,
					Name:  network.NicName,
					Match: match,
				},
			},
		},
	}

	return net
}

func (g GuestInfos) isEmpty() bool {
	return len(g) == 0
}

func (g GuestInfos) toExtraConfig() extraConfig {
	extraConfig := make(extraConfig, 0, len(g))

	for k, v := range g {
		extraConfig.Set(fmt.Sprintf("guestinfo.%s", k), v)
	}

	return extraConfig
}

func (e *extraConfig) String() string {
	return fmt.Sprintf("%v", *e)
}

func (e *extraConfig) Set(k, v string) {
	*e = append(*e, &types.OptionValue{Key: k, Value: v})
}

// VirtualMachine return govmomi virtual machine
func (vm *VirtualMachine) VirtualMachine(ctx *Context) *object.VirtualMachine {
	key := vm.Ref.String()

	if v := ctx.Value(key); v != nil {
		return v.(*object.VirtualMachine)
	}

	f := vm.Datastore.Datacenter.NewFinder(ctx)

	v, err := f.ObjectReference(ctx, vm.Ref)

	if err != nil {
		glog.Fatalf("Can't find virtual machine:%s", vm.Name)
	}

	//	v := object.NewVirtualMachine(vm.VimClient(), vm.Ref)

	ctx.WithValue(key, v)
	ctx.WithValue(fmt.Sprintf("[%s] %s", vm.Datastore.Name, vm.Name), v)

	return v.(*object.VirtualMachine)
}

// VimClient return the VIM25 client
func (vm *VirtualMachine) VimClient() *vim25.Client {
	return vm.Datastore.VimClient()
}

// Configure set characteristic of VM a virtual machine
func (vm *VirtualMachine) Configure(ctx *Context, userName, authKey string, cloudInit interface{}, network *Network, annotation string, memory int, cpus int, disk int) error {
	var err error
	var task *object.Task

	if cpus > 0 || memory > 0 || len(annotation) > 0 {
		virtualMachine := vm.VirtualMachine(ctx)

		vmConfigSpec := types.VirtualMachineConfigSpec{}

		if cpus > 0 {
			vmConfigSpec.NumCPUs = int32(cpus)
		}

		if memory > 0 {
			vmConfigSpec.MemoryMB = int64(memory)
		}

		vmConfigSpec.Annotation = annotation

		if guestInfos, err := vm.cloudInit(ctx, vm.Name, userName, authKey, cloudInit, network); err != nil {
			if guestInfos.isEmpty() == false {
				vmConfigSpec.ExtraConfig = guestInfos.toExtraConfig()
			}
		}

		if task, err = virtualMachine.Reconfigure(ctx, vmConfigSpec); err == nil {
			_, err = task.WaitForResult(ctx, nil)
		}
	}

	return nil
}

// WaitForIP wait ip
func (vm *VirtualMachine) WaitForIP(ctx *Context) (string, error) {
	var powerState types.VirtualMachinePowerState
	var err error
	var ip string

	v := vm.VirtualMachine(ctx)

	if powerState, err = v.PowerState(ctx); err == nil {
		if powerState == types.VirtualMachinePowerStatePoweredOn {
			ip, err = v.WaitForIP(ctx)
		} else {
			err = fmt.Errorf("The VM: %s is not powered", v.InventoryPath)
		}
	}

	return ip, err
}

// PowerOn power on a virtual machine
func (vm *VirtualMachine) PowerOn(ctx *Context) error {
	var powerState types.VirtualMachinePowerState
	var err error
	var task *object.Task

	v := vm.VirtualMachine(ctx)

	if powerState, err = v.PowerState(ctx); err == nil {
		if powerState != types.VirtualMachinePowerStatePoweredOn {
			task, err = v.PowerOn(ctx)

			_, err = task.WaitForResult(ctx, nil)
		} else {
			err = fmt.Errorf("The VM: %s is already powered", v.InventoryPath)
		}
	}

	return err
}

// PowerOff power off a virtual machine
func (vm *VirtualMachine) PowerOff(ctx *Context) error {
	var powerState types.VirtualMachinePowerState
	var err error
	var task *object.Task

	v := vm.VirtualMachine(ctx)

	if powerState, err = v.PowerState(ctx); err == nil {
		if powerState == types.VirtualMachinePowerStatePoweredOn {
			task, err = v.PowerOff(ctx)

			_, err = task.WaitForResult(ctx, nil)
		} else {
			err = fmt.Errorf("The VM: %s is already power off", v.InventoryPath)
		}
	}

	return err
}

// Delete delete the virtual machine
func (vm *VirtualMachine) Delete(ctx *Context) error {
	var powerState types.VirtualMachinePowerState
	var err error
	var task *object.Task

	v := vm.VirtualMachine(ctx)

	if powerState, err = v.PowerState(ctx); err == nil {
		if powerState != types.VirtualMachinePowerStatePoweredOn {
			task, err = v.Destroy(ctx)

			_, err = task.WaitForResult(ctx, nil)
		} else {
			err = fmt.Errorf("The VM: %s is powered", v.InventoryPath)
		}
	}

	return err
}

// Status refresh status virtual machine
func (vm *VirtualMachine) Status(ctx *Context) (*Status, error) {
	var powerState types.VirtualMachinePowerState
	var err error
	var status *Status

	v := vm.VirtualMachine(ctx)

	if powerState, err = v.PowerState(ctx); err == nil {
		address := ""

		if powerState == types.VirtualMachinePowerStatePoweredOn {
			address, err = v.WaitForIP(ctx)
		}

		status = &Status{
			Address: address,
			Powered: powerState == types.VirtualMachinePowerStatePoweredOn,
		}
	}

	return status, err
}

// SetGuestInfo change guest ingos
func (vm *VirtualMachine) SetGuestInfo(ctx *Context, guestInfos *GuestInfos) error {
	var task *object.Task
	var err error

	vmConfigSpec := types.VirtualMachineConfigSpec{}
	v := vm.VirtualMachine(ctx)

	if guestInfos != nil && guestInfos.isEmpty() == false {
		vmConfigSpec.ExtraConfig = guestInfos.toExtraConfig()
	} else {
		vmConfigSpec.ExtraConfig = []types.BaseOptionValue{}
	}

	if task, err = v.Reconfigure(ctx, vmConfigSpec); err == nil {
		err = task.Wait(ctx)
	}

	return err
}

func (vm *VirtualMachine) cloudInit(ctx *Context, hostName string, userName, authKey string, cloudInit interface{}, network *Network) (*GuestInfos, error) {
	var metadata, userdata, vendordata, netconfig string
	var err error
	var guestInfos *GuestInfos

	v := vm.VirtualMachine(ctx)

	// Only DHCP supported
	if network != nil && len(network.NicName) > 0 {
		if netconfig, err = encodeObject("networkconfig", buildNetworkConfig(network)); err != nil {
			err = fmt.Errorf(constantes.ErrUnableToEncodeGuestInfo, "networkconfig", err)
		} else if metadata, err = encodeObject("metadata", map[string]string{
			"network":          netconfig,
			"network.encoding": "gzip+base64",
			"local-hostname":   hostName,
			"instance-id":      v.UUID(ctx),
		}); err != nil {
			err = fmt.Errorf(constantes.ErrUnableToEncodeGuestInfo, "metadata", err)
		}
	} else if metadata, _ = encodeObject("metadata", map[string]string{
		"local-hostname": hostName,
		"instance-id":    v.UUID(ctx),
	}); err != nil {
		err = fmt.Errorf(constantes.ErrUnableToEncodeGuestInfo, "metadata", err)
	}

	if err == nil {

		if cloudInit != nil {
			if userdata, err = encodeObject("userdata", cloudInit); err != nil {
				return nil, fmt.Errorf(constantes.ErrUnableToEncodeGuestInfo, "userdata", err)
			}
		} else if userdata, err = encodeObject("userdata", map[string]string{}); err != nil {
			return nil, fmt.Errorf(constantes.ErrUnableToEncodeGuestInfo, "userdata", err)
		}

		if len(userName) > 0 && len(authKey) > 0 {
			if vendordata, err = encodeObject("vendordata", buildVendorData(userName, authKey)); err != nil {
				return nil, fmt.Errorf(constantes.ErrUnableToEncodeGuestInfo, "vendordata", err)
			}
		} else if vendordata, err = encodeObject("vendordata", map[string]string{}); err != nil {
			return nil, fmt.Errorf(constantes.ErrUnableToEncodeGuestInfo, "vendordata", err)
		}

		guestInfos = &GuestInfos{
			"metadata":            metadata,
			"metadata.encoding":   "gzip+base64",
			"userdata":            userdata,
			"userdata.encoding":   "gzip+base64",
			"vendordata":          vendordata,
			"vendordata.encoding": "gzip+base64",
		}
	}

	return guestInfos, err
}