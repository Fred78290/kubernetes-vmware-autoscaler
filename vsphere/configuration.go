package vsphere

import (
	"fmt"
	"net/url"
	"time"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/soap"
)

// Configuration declares vsphere connection info
type Configuration struct {
	Host          string        `json:"host"`
	UserName      string        `json:"uid"`
	Password      string        `json:"password"`
	Insecure      bool          `json:"insecure"`
	DataCenter    string        `json:"dc"`
	DataStore     string        `json:"datastore"`
	Resource      string        `json:"resource"`
	VMBasePath    string        `json:"vm-base-path"`
	Timeout       time.Duration `json:"timeout"`
	TemplateName  string        `json:"template-name"`
	Template      bool          `json:"template"`
	LinkedClone   bool          `json:"linked"`
	Customization string        `json:"Customization"`
	Network       *Network      `json:"network"`
}

// Status shortened vm status
type Status struct {
	Address string
	Powered bool
}

func (conf *Configuration) getURL() string {
	return fmt.Sprintf("https://%s:%s@%s%s", conf.UserName, conf.Password, conf.Host, vim25.Path)
}

// GetClient create a new govomi client
func (conf *Configuration) GetClient(ctx *Context) (*Client, error) {
	var u *url.URL
	var err error
	var c *govmomi.Client

	if u, err = soap.ParseURL(conf.getURL()); err == nil {
		// Connect and log in to ESX or vCenter
		if c, err = govmomi.NewClient(ctx, u, conf.Insecure); err == nil {
			return &Client{
				Client:        c,
				Configuration: conf,
			}, nil
		}
	}

	return nil, err
}

// Create will create a named VM not powered
func (conf *Configuration) Create(name string, guestInfos *GuestInfos, annotation string, memory int, cpus int, disk int) (*VirtualMachine, error) {
	var err error
	var client *Client
	var dc *Datacenter
	var ds *Datastore
	var vm *VirtualMachine

	ctx := NewContext(conf.Timeout)
	defer ctx.Cancel()

	if client, err = conf.GetClient(ctx); err == nil {
		if dc, err = client.GetDatacenter(ctx, conf.DataCenter); err == nil {
			if ds, err = dc.GetDatastore(ctx, conf.DataStore); err == nil {
				if vm, err = ds.CreateVirtualMachine(ctx, name); err == nil {
					err = vm.Configure(ctx, guestInfos, annotation, memory, cpus, disk)
				}
			}
		}
	}

	return vm, err
}

// Delete a VM by name
func (conf *Configuration) Delete(name string) error {
	ctx := NewContext(conf.Timeout)
	defer ctx.Cancel()

	vm, err := conf.VirtualMachine(ctx, name)

	if err != nil {
		return err
	}

	return vm.Delete(ctx)
}

// VirtualMachine  Retrieve VM by name
func (conf *Configuration) VirtualMachine(ctx *Context, name string) (*VirtualMachine, error) {
	var err error
	var client *Client
	var dc *Datacenter
	var ds *Datastore

	if client, err = conf.GetClient(ctx); err == nil {
		if dc, err = client.GetDatacenter(ctx, conf.DataCenter); err == nil {
			if ds, err = dc.GetDatastore(ctx, conf.DataStore); err == nil {
				return ds.GetVirtualMachine(ctx, name)
			}
		}
	}

	return nil, err
}

// PowerOn power on a VM by name
func (conf *Configuration) PowerOn(name string) error {
	ctx := NewContext(conf.Timeout)
	defer ctx.Cancel()

	vm, err := conf.VirtualMachine(ctx, name)

	if err != nil {
		return err
	}

	return vm.PowerOn(ctx)
}

// WaitForIP wait ip a VM by name
func (conf *Configuration) WaitForIP(name string) (string, error) {
	ctx := NewContext(conf.Timeout)
	defer ctx.Cancel()

	vm, err := conf.VirtualMachine(ctx, name)

	if err != nil {
		return "", err
	}

	return vm.WaitForIP(ctx)
}

// PowerOff power off a VM by name
func (conf *Configuration) PowerOff(name string) error {
	ctx := NewContext(conf.Timeout)
	defer ctx.Cancel()

	vm, err := conf.VirtualMachine(ctx, name)

	if err != nil {
		return err
	}

	return vm.PowerOff(ctx)
}

// Status return the current status of VM by name
func (conf *Configuration) Status(name string) (*Status, error) {
	ctx := NewContext(conf.Timeout)
	defer ctx.Cancel()

	vm, err := conf.VirtualMachine(ctx, name)

	if err != nil {
		return nil, err
	}

	return vm.Status(ctx)
}
