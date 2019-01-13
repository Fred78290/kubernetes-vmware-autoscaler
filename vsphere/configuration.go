package vsphere

import (
	"net/url"
	"time"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/vim25/soap"
)

// Configuration declares vsphere connection info
type Configuration struct {
	URL           string        `json:"url"`
	UserName      string        `json:"uid"`
	Password      string        `json:"password"`
	Insecure      bool          `json:"insecure"`
	DataCenter    string        `json:"dc"`
	DataStore     string        `json:"datastore"`
	Resource      string        `json:"resource-pool"`
	VMBasePath    string        `json:"vmFolder"`
	Timeout       time.Duration `json:"timeout"`
	TemplateName  string        `json:"template-name"`
	Template      bool          `json:"template"`
	LinkedClone   bool          `json:"linked"`
	Customization string        `json:"customization"`
	Network       *Network      `json:"network"`
}

// Status shortened vm status
type Status struct {
	Address string
	Powered bool
}

func (conf *Configuration) getURL() (string, error) {
	u, err := url.Parse(conf.URL)

	if err != nil {
		return "", err
	}

	u.User = url.UserPassword(conf.UserName, conf.Password)

	return u.String(), err
}

// GetClient create a new govomi client
func (conf *Configuration) GetClient(ctx *Context) (*Client, error) {
	var u *url.URL
	var sURL string
	var err error
	var c *govmomi.Client

	if sURL, err = conf.getURL(); err == nil {
		if u, err = soap.ParseURL(sURL); err == nil {
			// Connect and log in to ESX or vCenter
			if c, err = govmomi.NewClient(ctx, u, conf.Insecure); err == nil {
				return &Client{
					Client:        c,
					Configuration: conf,
				}, nil
			}
		}
	}
	return nil, err
}

// CreateWithContext will create a named VM not powered
// memory and disk are in megabytes
func (conf *Configuration) CreateWithContext(ctx *Context, name string, userName, authKey string, cloudInit interface{}, network *Network, annotation string, memory int, cpus int, disk int) (*VirtualMachine, error) {
	var err error
	var client *Client
	var dc *Datacenter
	var ds *Datastore
	var vm *VirtualMachine

	if client, err = conf.GetClient(ctx); err == nil {
		if dc, err = client.GetDatacenter(ctx, conf.DataCenter); err == nil {
			if ds, err = dc.GetDatastore(ctx, conf.DataStore); err == nil {
				if vm, err = ds.CreateVirtualMachine(ctx, name, conf.TemplateName, conf.VMBasePath, conf.Resource, conf.Template, conf.LinkedClone, network, conf.Customization); err == nil {
					err = vm.Configure(ctx, userName, authKey, cloudInit, network, annotation, memory, cpus, disk)
				}
			}
		}
	}

	// If an error occured delete VM
	if err != nil && vm != nil {
		vm.Delete(ctx)
	}

	return vm, err
}

// Create will create a named VM not powered
// memory and disk are in megabytes
func (conf *Configuration) Create(name string, userName, authKey string, cloudInit interface{}, network *Network, annotation string, memory int, cpus int, disk int) (*VirtualMachine, error) {
	ctx := NewContext(conf.Timeout)
	defer ctx.Cancel()

	return conf.CreateWithContext(ctx, name, userName, authKey, cloudInit, network, annotation, memory, cpus, disk)
}

// DeleteWithContext a VM by name
func (conf *Configuration) DeleteWithContext(ctx *Context, name string) error {
	vm, err := conf.VirtualMachineWithContext(ctx, name)

	if err != nil {
		return err
	}

	return vm.Delete(ctx)
}

// Delete a VM by name
func (conf *Configuration) Delete(name string) error {
	ctx := NewContext(conf.Timeout)
	defer ctx.Cancel()

	return conf.DeleteWithContext(ctx, name)
}

// VirtualMachineWithContext  Retrieve VM by name
func (conf *Configuration) VirtualMachineWithContext(ctx *Context, name string) (*VirtualMachine, error) {
	var err error
	var client *Client
	var dc *Datacenter
	var ds *Datastore

	if client, err = conf.GetClient(ctx); err == nil {
		if dc, err = client.GetDatacenter(ctx, conf.DataCenter); err == nil {
			if ds, err = dc.GetDatastore(ctx, conf.DataStore); err == nil {
				return ds.VirtualMachine(ctx, name)
			}
		}
	}

	return nil, err
}

// VirtualMachine  Retrieve VM by name
func (conf *Configuration) VirtualMachine(name string) (*VirtualMachine, error) {
	ctx := NewContext(conf.Timeout)
	defer ctx.Cancel()

	return conf.VirtualMachineWithContext(ctx, name)
}

// VirtualMachineListWithContext return all VM for the current datastore
func (conf *Configuration) VirtualMachineListWithContext(ctx *Context) ([]*VirtualMachine, error) {
	var err error
	var client *Client
	var dc *Datacenter
	var ds *Datastore

	if client, err = conf.GetClient(ctx); err == nil {
		if dc, err = client.GetDatacenter(ctx, conf.DataCenter); err == nil {
			if ds, err = dc.GetDatastore(ctx, conf.DataStore); err == nil {
				return ds.List(ctx)
			}
		}
	}

	return nil, err
}

// VirtualMachineList return all VM for the current datastore
func (conf *Configuration) VirtualMachineList() ([]*VirtualMachine, error) {
	ctx := NewContext(conf.Timeout)
	defer ctx.Cancel()

	return conf.VirtualMachineListWithContext(ctx)
}

// WaitForIPWithContext wait ip a VM by name
func (conf *Configuration) WaitForIPWithContext(ctx *Context, name string) (string, error) {
	vm, err := conf.VirtualMachineWithContext(ctx, name)

	if err != nil {
		return "", err
	}

	return vm.WaitForIP(ctx)
}

// WaitForIP wait ip a VM by name
func (conf *Configuration) WaitForIP(name string) (string, error) {
	ctx := NewContext(conf.Timeout)
	defer ctx.Cancel()

	return conf.WaitForIPWithContext(ctx, name)
}

// PowerOnWithContext power on a VM by name
func (conf *Configuration) PowerOnWithContext(ctx *Context, name string) error {
	vm, err := conf.VirtualMachineWithContext(ctx, name)

	if err != nil {
		return err
	}

	return vm.PowerOn(ctx)
}

// PowerOn power on a VM by name
func (conf *Configuration) PowerOn(name string) error {
	ctx := NewContext(conf.Timeout)
	defer ctx.Cancel()

	return conf.PowerOnWithContext(ctx, name)
}

// PowerOffWithContext power off a VM by name
func (conf *Configuration) PowerOffWithContext(ctx *Context, name string) error {
	vm, err := conf.VirtualMachineWithContext(ctx, name)

	if err != nil {
		return err
	}

	return vm.ShutdownGuest(ctx)
}

// PowerOff power off a VM by name
func (conf *Configuration) PowerOff(name string) error {
	ctx := NewContext(conf.Timeout)
	defer ctx.Cancel()

	return conf.PowerOffWithContext(ctx, name)
}

// StatusWithContext return the current status of VM by name
func (conf *Configuration) StatusWithContext(ctx *Context, name string) (*Status, error) {
	vm, err := conf.VirtualMachineWithContext(ctx, name)

	if err != nil {
		return nil, err
	}

	return vm.Status(ctx)
}

// Status return the current status of VM by name
func (conf *Configuration) Status(name string) (*Status, error) {
	ctx := NewContext(conf.Timeout)
	defer ctx.Cancel()

	return conf.StatusWithContext(ctx, name)
}
