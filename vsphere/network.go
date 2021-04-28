package vsphere

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/Fred78290/kubernetes-vmware-autoscaler/context"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// NetworkInterface declare single interface
type NetworkInterface struct {
	Existing         bool   `json:"exists,omitempty" yaml:"primary,omitempty"`
	NetworkName      string `json:"network,omitempty" yaml:"network,omitempty"`
	Adapter          string `json:"adapter,omitempty" yaml:"adapter,omitempty"`
	MacAddress       string `json:"mac-address,omitempty" yaml:"mac-address,omitempty"`
	NicName          string `json:"nic,omitempty" yaml:"nic,omitempty"`
	DHCP             bool   `json:"dhcp,omitempty" yaml:"dhcp,omitempty"`
	IPAddress        string `json:"address,omitempty" yaml:"address,omitempty"`
	Netmask          string `json:"netmask,omitempty" yaml:"netmask,omitempty"`
	Gateway          string `json:"gateway,omitempty" yaml:"gateway,omitempty"`
	networkReference object.NetworkReference
	networkBacking   types.BaseVirtualDeviceBackingInfo
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

// Converts IP mask to 16 bit unsigned integer.
func addressToInteger(mask net.IP) uint32 {
	var i uint32

	buf := bytes.NewReader(mask)

	_ = binary.Read(buf, binary.BigEndian, &i)

	return i
}

// ToCIDR returns address in cidr format ww.xx.yy.zz/NN
func ToCIDR(address, netmask string) string {

	if len(netmask) == 0 {
		mask := net.ParseIP(address).DefaultMask()
		netmask = net.IPv4(mask[0], mask[1], mask[2], mask[3]).To4().String()
	}

	mask := net.ParseIP(netmask)
	netmask = strconv.FormatUint(uint64(addressToInteger(mask.To4())), 2)

	return fmt.Sprintf("%s/%d", address, strings.Count(netmask, "1"))
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
					Addresses: &[]string{
						ToCIDR(n.IPAddress, n.Netmask),
					},
				}
			}

			if len(macAddress) != 0 {
				ethernet.Match = &map[string]string{
					"macaddress": macAddress,
				}

				if len(n.NicName) > 0 {
					ethernet.NicName = &n.NicName
				}
			} else {
				ethernet.NicName = nil
			}

			if len(n.Gateway) > 0 {
				ethernet.Gateway4 = &n.Gateway
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

// GetDeclaredExistingInterfaces return the declared existing interfaces
func (net *Network) GetDeclaredExistingInterfaces() []*NetworkInterface {

	infs := make([]*NetworkInterface, 0, len(net.Interfaces))
	for _, inf := range net.Interfaces {
		if inf.Existing {
			infs = append(infs, inf)
		}
	}

	return infs
}

// Devices return all devices
func (net *Network) Devices(ctx *context.Context, devices object.VirtualDeviceList, dc *Datacenter) (object.VirtualDeviceList, error) {
	var err error
	var device types.BaseVirtualDevice

	for _, n := range net.Interfaces {
		if !n.Existing {
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

	return fmt.Sprintf("00:16:3e:%02x:%02x:%02x", buf[0], buf[1], buf[2])
}

// See func (p DistributedVirtualPortgroup) EthernetCardBackingInfo(ctx context.Context) (types.BaseVirtualDeviceBackingInfo, error)
// Lack permissions workaround
func distributedVirtualPortgroupEthernetCardBackingInfo(ctx *context.Context, p *object.DistributedVirtualPortgroup) (string, error) {
	var dvp mo.DistributedVirtualPortgroup

	prop := "config.distributedVirtualSwitch"

	if err := p.Properties(ctx, p.Reference(), []string{"key", prop}, &dvp); err != nil {
		return "", err
	}

	return dvp.Key, nil
}

// MatchInterface return if this interface match the virtual device
// Due missing read permission, I can't create BackingInfo network card, so I use collected info to construct backing info
func (net *NetworkInterface) MatchInterface(ctx *context.Context, dc *Datacenter, card *types.VirtualEthernetCard) (bool, error) {

	equal := false

	if network, err := net.Reference(ctx, dc); err == nil {

		ref := network.Reference()

		if ref.Type == "Network" {
			if backing, ok := card.Backing.(*types.VirtualEthernetCardNetworkBackingInfo); ok {
				if c, err := network.EthernetCardBackingInfo(ctx); err == nil {
					if cc, ok := c.(*types.VirtualEthernetCardNetworkBackingInfo); ok {
						equal = backing.DeviceName == cc.DeviceName

						if equal {
							net.networkBacking = &types.VirtualEthernetCardNetworkBackingInfo{
								VirtualDeviceDeviceBackingInfo: types.VirtualDeviceDeviceBackingInfo{
									DeviceName: backing.DeviceName,
								},
							}
						}
					}
				} else {
					return false, err
				}
			}
		} else if ref.Type == "OpaqueNetwork" {
			if backing, ok := card.Backing.(*types.VirtualEthernetCardOpaqueNetworkBackingInfo); ok {
				if c, err := network.EthernetCardBackingInfo(ctx); err == nil {
					if cc, ok := c.(*types.VirtualEthernetCardOpaqueNetworkBackingInfo); ok {
						equal = backing.OpaqueNetworkId == cc.OpaqueNetworkId && backing.OpaqueNetworkType == cc.OpaqueNetworkType

						if equal {
							net.networkBacking = &types.VirtualEthernetCardOpaqueNetworkBackingInfo{
								OpaqueNetworkId:   backing.OpaqueNetworkId,
								OpaqueNetworkType: backing.OpaqueNetworkType,
							}
						}
					}
				} else {
					return false, err
				}
			}
		} else if ref.Type == "DistributedVirtualPortgroup" {
			if backing, ok := card.Backing.(*types.VirtualEthernetCardDistributedVirtualPortBackingInfo); ok {
				if portgroupKey, err := distributedVirtualPortgroupEthernetCardBackingInfo(ctx, network.(*object.DistributedVirtualPortgroup)); err == nil {
					equal = backing.Port.PortgroupKey == portgroupKey

					if equal {
						net.networkBacking = &types.VirtualEthernetCardDistributedVirtualPortBackingInfo{
							Port: types.DistributedVirtualSwitchPortConnection{
								SwitchUuid:   backing.Port.SwitchUuid,
								PortgroupKey: backing.Port.PortgroupKey,
							},
						}
					}
				}
			} else {
				return false, err
			}
		}
	}

	return equal, nil
}

// GetMacAddress return a macaddress
func (net *NetworkInterface) GetMacAddress() string {
	address := net.MacAddress

	if strings.ToLower(address) == "generate" {
		address = generateMacAddress()
	} else if strings.ToLower(address) == "ignore" {
		address = ""
	}

	net.MacAddress = address

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
func (net *NetworkInterface) Reference(ctx *context.Context, dc *Datacenter) (object.NetworkReference, error) {
	var err error

	if net.networkReference == nil {

		f := dc.NewFinder(ctx)

		net.networkReference, err = f.NetworkOrDefault(ctx, net.NetworkName)
	}

	return net.networkReference, err
}

// Device return a device
func (net *NetworkInterface) Device(ctx *context.Context, dc *Datacenter) (types.BaseVirtualDevice, error) {
	var backing types.BaseVirtualDeviceBackingInfo

	network, err := net.Reference(ctx, dc)

	if err != nil {
		return nil, err
	}

	networkReference := network.Reference()
	backing, err = network.EthernetCardBackingInfo(ctx)

	if err != nil {
		strErr := err.Error()

		if strings.Contains(strErr, "no System.Read privilege on:") {
			if false {
				backing = &types.VirtualEthernetCardOpaqueNetworkBackingInfo{
					OpaqueNetworkType: networkReference.Type,
					OpaqueNetworkId:   networkReference.Value,
				}
			} else {
				backing = &types.VirtualEthernetCardNetworkBackingInfo{
					Network: &networkReference,
					VirtualDeviceDeviceBackingInfo: types.VirtualDeviceDeviceBackingInfo{
						DeviceName: net.NetworkName,
					},
				}
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

// ChangeAddress just the mac adress
func (net *NetworkInterface) ChangeAddress(card *types.VirtualEthernetCard) bool {
	macAddress := net.GetMacAddress()

	if len(macAddress) != 0 {
		card.Backing = net.networkBacking
		card.AddressType = string(types.VirtualEthernetCardMacTypeManual)
		card.MacAddress = macAddress

		return true
	}

	return false
}

// NeedToReconfigure tell that we must set the mac address
func (net *NetworkInterface) NeedToReconfigure() bool {
	return len(net.GetMacAddress()) != 0 && net.Existing
}
