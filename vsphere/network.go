package vsphere

import (
	"context"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/types"
)

// Network describes a card adapter
type Network struct {
	Name    string `json:"name"`
	Adapter string `json:"adapter"`
	Address string `json:"address"`
	NicName string `json:"nic"`
}

// Reference return the network reference
func (net *Network) Reference(client *vim25.Client, dc *object.Datacenter) (object.NetworkReference, error) {
	f := find.NewFinder(client, true)

	f.SetDatacenter(dc)

	return f.NetworkOrDefault(context.TODO(), net.Name)
}

// Device return a device
func (net *Network) Device(client *vim25.Client, dc *object.Datacenter) (types.BaseVirtualDevice, error) {
	network, err := net.Reference(client, dc)
	if err != nil {
		return nil, err
	}

	backing, err := network.EthernetCardBackingInfo(context.TODO())
	if err != nil {
		return nil, err
	}

	device, err := object.EthernetCardTypes().CreateEthernetCard(net.Adapter, backing)
	if err != nil {
		return nil, err
	}

	if net.Address != "" {
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
