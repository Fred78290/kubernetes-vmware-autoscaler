package vsphere

import (
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/types"
)

// Client Client wrapper
type Client struct {
	Client        *govmomi.Client
	Configuration *Configuration
}

// GetClient return client
func (c *Client) GetClient() *govmomi.Client {
	return c.Client
}

// VimClient return vim25 client
func (c *Client) VimClient() *vim25.Client {
	return c.Client.Client
}

// VirtualMachine map govmomi VirtualMachine
func (c *Client) VirtualMachine(ref types.ManagedObjectReference) *object.VirtualMachine {
	return object.NewVirtualMachine(c.VimClient(), ref)
}

// Datastore create govmomi Datastore object
func (c *Client) Datastore(ref types.ManagedObjectReference) *object.Datastore {
	return object.NewDatastore(c.VimClient(), ref)
}

// Datacenter return a Datacenter
func (c *Client) Datacenter(ref types.ManagedObjectReference) *object.Datacenter {
	return object.NewDatacenter(c.VimClient(), ref)
}

// GetDatacenter return the datacenter
func (c *Client) GetDatacenter(ctx *Context, name string) (*Datacenter, error) {
	var dc *object.Datacenter
	var err error

	f := find.NewFinder(c.Client.Client, true)

	if dc, err = f.Datacenter(ctx, name); err == nil {

		return &Datacenter{
			Ref:    dc.Reference(),
			Client: c,
		}, nil
	}

	return nil, err
}
