package vsphere

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Fred78290/kubernetes-vmware-autoscaler/constantes"
	"github.com/Fred78290/kubernetes-vmware-autoscaler/context"
	glog "github.com/sirupsen/logrus"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25"
	"gopkg.in/yaml.v2"

	"github.com/vmware/govmomi/vim25/mo"
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

func encodeMetadata(object interface{}) (string, error) {
	var result string
	out, err := json.Marshal(object)

	if err == nil {
		var stdout bytes.Buffer
		var zw = gzip.NewWriter(&stdout)

		zw.Name = "metadata"
		zw.ModTime = time.Now()

		if _, err = zw.Write(out); err == nil {
			if err = zw.Close(); err == nil {
				result = base64.StdEncoding.EncodeToString(stdout.Bytes())
			}
		}
	}

	return result, err
}

func encodeCloudInit(name string, object interface{}) (string, error) {
	var result string
	var out bytes.Buffer
	var err error

	fmt.Fprintln(&out, "#cloud-init")

	wr := yaml.NewEncoder(&out)
	err = wr.Encode(object)

	wr.Close()

	if err == nil {
		var stdout bytes.Buffer
		var zw = gzip.NewWriter(&stdout)

		zw.Name = name
		zw.ModTime = time.Now()

		if _, err = zw.Write(out.Bytes()); err == nil {
			if err = zw.Close(); err == nil {
				result = base64.StdEncoding.EncodeToString(stdout.Bytes())
			}
		}
	}

	return result, err
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

func (g GuestInfos) isEmpty() bool {
	return len(g) == 0
}

func (g GuestInfos) toExtraConfig() []types.BaseOptionValue {
	extraConfig := make([]types.BaseOptionValue, len(g))

	for k, v := range g {
		extraConfig = append(extraConfig,
			&types.OptionValue{
				Key:   fmt.Sprintf("guestinfo.%s", k),
				Value: v,
			})
	}

	return extraConfig
}

func (vm *VirtualMachine) HostSystem(ctx *context.Context) (string, error) {

	host, err := vm.VirtualMachine(ctx).HostSystem(ctx)

	if err != nil {
		return "*", err
	}

	return host.ObjectName(ctx)
}

// VirtualMachine return govmomi virtual machine
func (vm *VirtualMachine) VirtualMachine(ctx *context.Context) *object.VirtualMachine {
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

func (vm *VirtualMachine) collectNetworkInfos(ctx *context.Context, network *Network, nodeIndex int) error {
	virtualMachine := vm.VirtualMachine(ctx)
	devices, err := virtualMachine.Device(ctx)

	if err == nil {
		for _, device := range devices {
			// It's an ether device?
			if ethernet, ok := device.(types.BaseVirtualEthernetCard); ok {
				// Match my network?
				for _, inf := range network.Interfaces {
					card := ethernet.GetVirtualEthernetCard()
					if match, err := inf.MatchInterface(ctx, vm.Datastore.Datacenter, card); match && err == nil {
						inf.AttachMacAddress(card.MacAddress, nodeIndex)
					}
				}
			}
		}
	}

	return err
}

func (vm *VirtualMachine) addNetwork(ctx *context.Context, network *Network, devices object.VirtualDeviceList, nodeIndex int) (object.VirtualDeviceList, error) {
	var err error

	if network != nil && len(network.Interfaces) > 0 {
		devices, err = network.Devices(ctx, devices, vm.Datastore.Datacenter, nodeIndex)
	}

	return devices, err
}

func (vm *VirtualMachine) findHardDrive(ctx *context.Context, list object.VirtualDeviceList) (*types.VirtualDisk, error) {
	var disks []*types.VirtualDisk

	for _, device := range list {
		switch md := device.(type) {
		case *types.VirtualDisk:
			disks = append(disks, md)
		default:
			continue
		}
	}

	switch len(disks) {
	case 0:
		return nil, errors.New("no disk found using the given values")
	case 1:
		return disks[0], nil
	}

	return nil, errors.New("the given disk values match multiple disks")
}

func (vm *VirtualMachine) addOrExpandHardDrive(ctx *context.Context, virtualMachine *object.VirtualMachine, diskSize int, expandHardDrive bool, devices object.VirtualDeviceList) (object.VirtualDeviceList, error) {
	var err error
	var existingDevices object.VirtualDeviceList
	var controller types.BaseVirtualController

	if diskSize > 0 {
		drivePath := fmt.Sprintf("[%s] %s/harddrive.vmdk", vm.Datastore.Name, vm.Name)

		if existingDevices, err = virtualMachine.Device(ctx); err == nil {
			if expandHardDrive {
				var task *object.Task
				var disk *types.VirtualDisk

				if disk, err = vm.findHardDrive(ctx, existingDevices); err == nil {
					diskSizeInKB := int64(diskSize * 1024)

					if diskSizeInKB > disk.CapacityInKB {
						disk.CapacityInKB = diskSizeInKB

						spec := types.VirtualMachineConfigSpec{
							DeviceChange: []types.BaseVirtualDeviceConfigSpec{
								&types.VirtualDeviceConfigSpec{
									Device:    disk,
									Operation: types.VirtualDeviceConfigSpecOperationEdit,
								},
							},
						}

						if task, err = virtualMachine.Reconfigure(ctx, spec); err == nil {
							err = task.Wait(ctx)
						}
					}
				}
			} else {
				if controller, err = existingDevices.FindDiskController(""); err == nil {

					disk := existingDevices.CreateDisk(controller, vm.Datastore.Ref, drivePath)

					if len(existingDevices.SelectByBackingInfo(disk.Backing)) != 0 {

						err = fmt.Errorf("disk %s already exists", drivePath)

					} else {

						backing := disk.Backing.(*types.VirtualDiskFlatVer2BackingInfo)

						backing.ThinProvisioned = types.NewBool(true)
						backing.DiskMode = string(types.VirtualDiskModePersistent)
						backing.Sharing = string(types.VirtualDiskSharingSharingNone)

						disk.CapacityInKB = int64(diskSize) * 1024

						devices = append(devices, disk)
					}
				}
			}
		}
	}

	return devices, err
}

// Configure set characteristic of VM a virtual machine
func (vm *VirtualMachine) Configure(ctx *context.Context, userName, authKey string, cloudInit interface{}, network *Network, annotation string, expandHardDrive bool, memory, cpus, disk, nodeIndex int) error {
	var devices object.VirtualDeviceList
	var err error
	var task *object.Task

	virtualMachine := vm.VirtualMachine(ctx)

	vmConfigSpec := types.VirtualMachineConfigSpec{
		NumCPUs:      int32(cpus),
		MemoryMB:     int64(memory),
		Annotation:   annotation,
		InstanceUuid: virtualMachine.UUID(ctx),
		Uuid:         virtualMachine.UUID(ctx),
	}

	if devices, err = vm.addOrExpandHardDrive(ctx, virtualMachine, disk, expandHardDrive, devices); err != nil {

		err = fmt.Errorf(constantes.ErrUnableToAddHardDrive, vm.Name, err)

	} else if devices, err = vm.addNetwork(ctx, network, devices, nodeIndex); err != nil {

		err = fmt.Errorf(constantes.ErrUnableToAddNetworkCard, vm.Name, err)

	} else if vmConfigSpec.DeviceChange, err = devices.ConfigSpec(types.VirtualDeviceConfigSpecOperationAdd); err != nil {

		err = fmt.Errorf(constantes.ErrUnableToCreateDeviceChangeOp, vm.Name, err)

	} else if vmConfigSpec.ExtraConfig, err = vm.cloudInit(ctx, vm.Name, userName, authKey, cloudInit, network, nodeIndex); err != nil {

		err = fmt.Errorf(constantes.ErrCloudInitFailCreation, vm.Name, err)

	} else if task, err = virtualMachine.Reconfigure(ctx, vmConfigSpec); err != nil {

		err = fmt.Errorf(constantes.ErrUnableToReconfigureVM, vm.Name, err)

	} else if err = task.Wait(ctx); err != nil {

		err = fmt.Errorf(constantes.ErrUnableToReconfigureVM, vm.Name, err)

	}

	return err
}

func (vm VirtualMachine) isToolsRunning(ctx *context.Context, v *object.VirtualMachine) (bool, error) {
	var o mo.VirtualMachine

	running, err := v.IsToolsRunning(ctx)
	if err != nil {
		return false, err
	}

	if running {
		return running, nil
	}

	err = v.Properties(ctx, v.Reference(), []string{"guest"}, &o)
	if err != nil {
		return false, err
	}

	return o.Guest.ToolsRunningStatus == string(types.VirtualMachineToolsRunningStatusGuestToolsRunning), nil
}

// IsToolsRunning returns true if VMware Tools is currently running in the guest OS, and false otherwise.
func (vm VirtualMachine) IsToolsRunning(ctx *context.Context) (bool, error) {
	v := vm.VirtualMachine(ctx)

	return vm.isToolsRunning(ctx, v)
}

func (vm *VirtualMachine) waitForToolsRunning(ctx *context.Context, v *object.VirtualMachine) (bool, error) {
	var running bool

	p := property.DefaultCollector(vm.VimClient())

	err := property.Wait(ctx, p, v.Reference(), []string{"guest.toolsRunningStatus"}, func(pc []types.PropertyChange) bool {
		for _, c := range pc {
			if c.Name != "guest.toolsRunningStatus" {
				continue
			}
			if c.Op != types.PropertyChangeOpAssign {
				continue
			}
			if c.Val == nil {
				continue
			}

			running = c.Val.(string) == string(types.VirtualMachineToolsRunningStatusGuestToolsRunning)

			return running
		}

		return false
	})

	if err != nil {
		return false, err
	}

	return running, nil
}

func (vm *VirtualMachine) ListAddresses(ctx *context.Context) ([]NetworkInterface, error) {
	var o mo.VirtualMachine

	v := vm.VirtualMachine(ctx)

	if err := v.Properties(ctx, v.Reference(), []string{"guest"}, &o); err != nil {
		return nil, err
	}

	addresses := make([]NetworkInterface, 0, len(o.Guest.Net))

	for _, net := range o.Guest.Net {
		if net.Connected {
			addresses = append(addresses, NetworkInterface{
				NetworkName: net.Network,
				MacAddress:  net.MacAddress,
				IPAddress:   net.IpAddress[0],
			})
		}
	}

	return addresses, nil
}

// WaitForToolsRunning wait vmware tool starts
func (vm *VirtualMachine) WaitForToolsRunning(ctx *context.Context) (bool, error) {
	var powerState types.VirtualMachinePowerState
	var err error
	var running bool

	v := vm.VirtualMachine(ctx)

	if powerState, err = v.PowerState(ctx); err == nil {
		if powerState == types.VirtualMachinePowerStatePoweredOn {
			running, err = vm.waitForToolsRunning(ctx, v)
		} else {
			err = fmt.Errorf("the VM: %s is not powered", v.InventoryPath)
		}
	}

	return running, err
}

func (vm *VirtualMachine) waitForIP(ctx *context.Context, v *object.VirtualMachine) (string, error) {
	if running, err := vm.isToolsRunning(ctx, v); running {
		return v.WaitForIP(ctx)
	} else if err != nil {
		return "", err
	}

	return "", nil
}

// WaitForIP wait ip
func (vm *VirtualMachine) WaitForIP(ctx *context.Context) (string, error) {
	var powerState types.VirtualMachinePowerState
	var err error
	var ip string

	v := vm.VirtualMachine(ctx)

	if powerState, err = v.PowerState(ctx); err == nil {
		if powerState == types.VirtualMachinePowerStatePoweredOn {
			ip, err = vm.waitForIP(ctx, v)
		} else {
			err = fmt.Errorf("the VM: %s is not powered", v.InventoryPath)
		}
	}

	return ip, err
}

// PowerOn power on a virtual machine
func (vm *VirtualMachine) PowerOn(ctx *context.Context) error {
	var powerState types.VirtualMachinePowerState
	var err error
	var task *object.Task

	v := vm.VirtualMachine(ctx)

	if powerState, err = v.PowerState(ctx); err == nil {
		if powerState != types.VirtualMachinePowerStatePoweredOn {
			if task, err = v.PowerOn(ctx); err == nil {
				err = task.Wait(ctx)
			}
		} else {
			err = fmt.Errorf("the VM: %s is already powered", v.InventoryPath)
		}
	}

	return err
}

// PowerOff power off a virtual machine
func (vm *VirtualMachine) PowerOff(ctx *context.Context) error {
	var powerState types.VirtualMachinePowerState
	var err error
	var task *object.Task

	v := vm.VirtualMachine(ctx)

	if powerState, err = v.PowerState(ctx); err == nil {
		if powerState == types.VirtualMachinePowerStatePoweredOn {
			if task, err = v.PowerOff(ctx); err == nil {
				err = task.Wait(ctx)
			}
		} else {
			err = fmt.Errorf("the VM: %s is already off", v.InventoryPath)
		}
	}

	return err
}

// ShutdownGuest power off a virtual machine
func (vm *VirtualMachine) ShutdownGuest(ctx *context.Context) error {
	var powerState types.VirtualMachinePowerState
	var err error
	var task *object.Task

	v := vm.VirtualMachine(ctx)

	if powerState, err = v.PowerState(ctx); err == nil {
		if powerState == types.VirtualMachinePowerStatePoweredOn {
			if err = v.ShutdownGuest(ctx); err != nil {
				if task, err = v.PowerOff(ctx); err == nil {
					err = task.Wait(ctx)
				}
			}
		} else {
			err = fmt.Errorf("the VM: %s is already power off", v.InventoryPath)
		}
	}

	return err
}

// Delete delete the virtual machine
func (vm *VirtualMachine) Delete(ctx *context.Context) error {
	var powerState types.VirtualMachinePowerState
	var err error
	var task *object.Task

	v := vm.VirtualMachine(ctx)

	if powerState, err = v.PowerState(ctx); err == nil {
		if powerState != types.VirtualMachinePowerStatePoweredOn {
			if task, err = v.Destroy(ctx); err == nil {
				err = task.Wait(ctx)
			}
		} else {
			err = fmt.Errorf("the VM: %s is powered", v.InventoryPath)
		}
	}

	return err
}

// Status refresh status virtual machine
func (vm *VirtualMachine) Status(ctx *context.Context) (*Status, error) {
	var powerState types.VirtualMachinePowerState
	var err error
	var status *Status = &Status{}
	var interfaces []NetworkInterface
	v := vm.VirtualMachine(ctx)

	if powerState, err = v.PowerState(ctx); err == nil {
		if interfaces, err = vm.ListAddresses(ctx); err == nil {
			status.Interfaces = interfaces
			status.Powered = powerState == types.VirtualMachinePowerStatePoweredOn
		}
	}

	return status, err
}

// SetGuestInfo change guest ingos
func (vm *VirtualMachine) SetGuestInfo(ctx *context.Context, guestInfos *GuestInfos) error {
	var task *object.Task
	var err error

	vmConfigSpec := types.VirtualMachineConfigSpec{}
	v := vm.VirtualMachine(ctx)

	if guestInfos != nil && !guestInfos.isEmpty() {
		vmConfigSpec.ExtraConfig = guestInfos.toExtraConfig()
	} else {
		vmConfigSpec.ExtraConfig = []types.BaseOptionValue{}
	}

	if task, err = v.Reconfigure(ctx, vmConfigSpec); err == nil {
		err = task.Wait(ctx)
	}

	return err
}

func (vm *VirtualMachine) cloudInit(ctx *context.Context, hostName string, userName, authKey string, cloudInit interface{}, network *Network, nodeIndex int) ([]types.BaseOptionValue, error) {
	var metadata, userdata, vendordata, netconfig string
	var err error
	var guestInfos *GuestInfos

	v := vm.VirtualMachine(ctx)

	// Only DHCP supported
	if network != nil && len(network.Interfaces) > 0 {
		if netconfig, err = encodeObject("networkconfig", network.GetCloudInitNetwork(nodeIndex)); err != nil {
			err = fmt.Errorf(constantes.ErrUnableToEncodeGuestInfo, "networkconfig", err)
		} else if metadata, err = encodeMetadata(map[string]string{
			"network":          netconfig,
			"network.encoding": "gzip+base64",
			"local-hostname":   hostName,
			"instance-id":      v.UUID(ctx),
		}); err != nil {
			err = fmt.Errorf(constantes.ErrUnableToEncodeGuestInfo, "metadata", err)
		}
	} else if metadata, _ = encodeMetadata(map[string]string{
		"local-hostname": hostName,
		"instance-id":    v.UUID(ctx),
	}); err != nil {
		err = fmt.Errorf(constantes.ErrUnableToEncodeGuestInfo, "metadata", err)
	}

	if err == nil {

		if cloudInit != nil {
			if userdata, err = encodeCloudInit("userdata", cloudInit); err != nil {
				return nil, fmt.Errorf(constantes.ErrUnableToEncodeGuestInfo, "userdata", err)
			}
		} else if userdata, err = encodeCloudInit("userdata", map[string]string{}); err != nil {
			return nil, fmt.Errorf(constantes.ErrUnableToEncodeGuestInfo, "userdata", err)
		}

		if len(userName) > 0 && len(authKey) > 0 {
			if vendordata, err = encodeCloudInit("vendordata", buildVendorData(userName, authKey)); err != nil {
				return nil, fmt.Errorf(constantes.ErrUnableToEncodeGuestInfo, "vendordata", err)
			}
		} else if vendordata, err = encodeCloudInit("vendordata", map[string]string{}); err != nil {
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

	return guestInfos.toExtraConfig(), err
}
