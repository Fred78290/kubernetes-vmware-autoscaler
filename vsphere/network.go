package vsphere

import (
	"strings"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
)

// Network describes a card adapter
type Network struct {
	Name    string `json:"name,omitempty"`
	Adapter string `json:"adapter,omitempty"`
	Address string `json:"address,omitempty"`
	NicName string `json:"nic,omitempty"`
}

// Reference return the network reference
func (net *Network) Reference(ctx *Context, dc *Datacenter) (object.NetworkReference, error) {
	f := dc.NewFinder(ctx)

	return f.NetworkOrDefault(ctx, net.Name)
}

// Device return a device
func (net *Network) Device(ctx *Context, dc *Datacenter) (types.BaseVirtualDevice, error) {
	var backing types.BaseVirtualDeviceBackingInfo

	network, err := net.Reference(ctx, dc)

	if err != nil {
		return nil, err
	}

	backing, err = network.EthernetCardBackingInfo(ctx)

	if err != nil {
		strErr := err.Error()

		if strings.Contains(strErr, "no System.Read privilege on:") {

			backing = &types.VirtualEthernetCardNetworkBackingInfo{
				VirtualDeviceDeviceBackingInfo: types.VirtualDeviceDeviceBackingInfo{
					DeviceName: net.Name,
				},
			}

		} else {
			return nil, err
		}
	}

	device, err := object.EthernetCardTypes().CreateEthernetCard(net.Adapter, backing)
	if err != nil {
		return nil, err
	}

	// Connect the device
	device.GetVirtualDevice().Connectable = &types.VirtualDeviceConnectInfo{
		StartConnected:    true,
		AllowGuestControl: true,
		Connected:         true,
	}

	if len(net.Address) != 0 {
		card := device.(types.BaseVirtualEthernetCard).GetVirtualEthernetCard()
		card.AddressType = string(types.VirtualEthernetCardMacTypeManual)
		card.MacAddress = net.Address
	}

	return device, nil
}

// Change applies update backing and hardware address changes to the given network device.
func (net *Network) Change(device types.BaseVirtualDevice, update types.BaseVirtualDevice) {
	current := device.(types.BaseVirtualEthernetCard).GetVirtualEthernetCard()
	changed := update.(types.BaseVirtualEthernetCard).GetVirtualEthernetCard()

	current.Backing = changed.Backing

	if changed.MacAddress != "" {
		current.MacAddress = changed.MacAddress
	}

	if changed.AddressType != "" {
		current.AddressType = changed.AddressType
	}
}
