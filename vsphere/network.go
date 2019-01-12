package vsphere

import (
	"crypto/rand"
	"fmt"
	"strings"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
)

// NetworkInterface declare single interface
type NetworkInterface struct {
	Primary     bool   `json:"primary,omitempty" yaml:"primary,omitempty"`
	NetworkName string `json:"network,omitempty" yaml:"network,omitempty"`
	Adapter     string `json:"adapter,omitempty" yaml:"adapter,omitempty"`
	MacAddress  string `json:"mac-address,omitempty" yaml:"mac-address,omitempty"`
	NicName     string `json:"nic,omitempty" yaml:"nic,omitempty"`
	DHCP        bool   `json:"dhcp,omitempty" yaml:"dhcp,omitempty"`
	IPAddress   string `json:"address,omitempty" yaml:"address,omitempty"`
	Gateway     string `json:"gateway,omitempty" yaml:"gateway,omitempty"`
}

// NetworkResolv /etc/resolv.conf
type NetworkResolv struct {
	Search     []string `json:"search,omitempty" yaml:"search,omitempty"`
	Nameserver []string `json:"nameserver,omitempty" yaml:"nameserver,omitempty"`
}

// Network describes a card adapter
type Network struct {
	Interfaces []*NetworkInterface `json:"interfaces,omitempty" yaml:"interfaces,omitempty"`
	DNS        *NetworkResolv      `json:"dns,omitempty" yaml:"dns,omitempty"`
}

// Nameserver declaration
type Nameserver struct {
	Search    []string `json:"search,omitempty" yaml:"search,omitempty"`
	Addresses []string `json:"addresses,omitempty" yaml:"addresses,omitempty"`
}

// NetworkAdapter wrapper
type NetworkAdapter struct {
	DHCP4       bool               `json:"dhcp4,omitempty" yaml:"dhcp4,omitempty"`
	NicName     *string            `json:"set-name,omitempty" yaml:"set-name,omitempty"`
	Match       *map[string]string `json:"match,omitempty" yaml:"match,omitempty"`
	Gateway4    *string            `json:"gateway4,omitempty" yaml:"gateway4,omitempty"`
	Addresses   *[]string          `json:"addresses,omitempty" yaml:"addresses,omitempty"`
	Nameservers *Nameserver        `json:"nameservers,omitempty" yaml:"nameservers,omitempty"`
}

// NetworkDeclare wrapper
type NetworkDeclare struct {
	Version   int                        `json:"version,omitempty" yaml:"version,omitempty"`
	Ethernets map[string]*NetworkAdapter `json:"ethernets,omitempty" yaml:"ethernets,omitempty"`
}

// NetworkConfig wrapper
type NetworkConfig struct {
	Network *NetworkDeclare `json:"network,omitempty" yaml:"network,omitempty"`
}

// GetCloudInitNetwork create cloud-init object
func (net *Network) GetCloudInitNetwork() *NetworkConfig {

	declare := &NetworkDeclare{
		Version:   2,
		Ethernets: make(map[string]*NetworkAdapter, len(net.Interfaces)),
	}

	for _, n := range net.Interfaces {
		if len(n.NicName) > 0 {
			var ethernet *NetworkAdapter
			var macAddress = n.GetMacAddress()

			if n.DHCP || len(n.IPAddress) == 0 {
				ethernet = &NetworkAdapter{
					DHCP4: n.DHCP,
				}
			} else {
				ethernet = &NetworkAdapter{
					Addresses: &[]string{n.IPAddress},
				}
			}

			if len(macAddress) != 0 {
				ethernet.Match = &map[string]string{
					"macaddress": macAddress,
				}
			}

			if len(n.Gateway) > 0 {
				ethernet.Gateway4 = &n.Gateway
			}

			if len(n.NicName) > 0 {
				ethernet.NicName = &n.NicName
			}

			if net.DNS != nil {
				ethernet.Nameservers = &Nameserver{
					Addresses: net.DNS.Nameserver,
					Search:    net.DNS.Search,
				}
			}

			declare.Ethernets[n.NicName] = ethernet
		}
	}

	return &NetworkConfig{
		Network: declare,
	}
}

// GetPrimaryInterface return the primary interface
func (net *Network) GetPrimaryInterface() *NetworkInterface {

	for _, inf := range net.Interfaces {
		if inf.Primary {
			return inf
		}
	}

	return nil
}

// Devices return all devices
func (net *Network) Devices(ctx *Context, devices object.VirtualDeviceList, dc *Datacenter) (object.VirtualDeviceList, error) {
	var err error
	var device types.BaseVirtualDevice

	for _, n := range net.Interfaces {
		if n.Primary == false {
			if device, err = n.Device(ctx, dc); err == nil {
				devices = append(devices, n.SetMacAddress(device))
			} else {
				break
			}
		}
	}

	return devices, err
}

func generateMacAddress() string {
	buf := make([]byte, 3)

	if _, err := rand.Read(buf); err != nil {
		return ""
	}

	return fmt.Sprintf("00:50:56:%02x:%02x:%02x", buf[0], buf[1], buf[2])
}

// GetMacAddress return a macaddress
func (net *NetworkInterface) GetMacAddress() string {
	address := net.MacAddress

	if strings.ToLower(address) == "generate" {
		address = generateMacAddress()
		net.MacAddress = address
	}

	return address
}

// SetMacAddress put mac address in the device
func (net *NetworkInterface) SetMacAddress(device types.BaseVirtualDevice) types.BaseVirtualDevice {
	adress := net.GetMacAddress()

	if len(adress) != 0 {
		card := device.(types.BaseVirtualEthernetCard).GetVirtualEthernetCard()
		card.AddressType = string(types.VirtualEthernetCardMacTypeManual)
		card.MacAddress = adress
	}

	return device
}

// Reference return the network reference
func (net *NetworkInterface) Reference(ctx *Context, dc *Datacenter) (object.NetworkReference, error) {
	f := dc.NewFinder(ctx)

	return f.NetworkOrDefault(ctx, net.NetworkName)
}

// Device return a device
func (net *NetworkInterface) Device(ctx *Context, dc *Datacenter) (types.BaseVirtualDevice, error) {
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
					DeviceName: net.NetworkName,
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

	macAddress := net.GetMacAddress()

	if len(macAddress) != 0 {
		card := device.(types.BaseVirtualEthernetCard).GetVirtualEthernetCard()
		card.AddressType = string(types.VirtualEthernetCardMacTypeManual)
		card.MacAddress = macAddress
	}

	return device, nil
}

// Change applies update backing and hardware address changes to the given network device.
func (net *NetworkInterface) Change(device types.BaseVirtualDevice, update types.BaseVirtualDevice) {
	current := device.(types.BaseVirtualEthernetCard).GetVirtualEthernetCard()
	changed := update.(types.BaseVirtualEthernetCard).GetVirtualEthernetCard()

	current.Backing = changed.Backing

	if len(changed.MacAddress) > 0 {
		current.MacAddress = changed.MacAddress
	}

	if len(changed.AddressType) > 0 {
		current.AddressType = changed.AddressType
	}
}
