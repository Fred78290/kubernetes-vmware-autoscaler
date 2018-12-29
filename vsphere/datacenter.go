package vsphere

import (
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/types"
)

var datacenterKey = contextKey("datacenter")

// Datacenter represent a datacenter
type Datacenter struct {
	Ref    types.ManagedObjectReference
	Client *Client
}

// Datacenter return a Datacenter
func (dc *Datacenter) Datacenter(ctx *Context) *object.Datacenter {
	if d := ctx.Value(datacenterKey); d != nil {
		return d.(*object.Datacenter)
	}

	d := object.NewDatacenter(dc.VimClient(), dc.Ref)
	ctx.WithValue(datacenterKey, d)

	return d
}

// VimClient return the VIM25 client
func (dc *Datacenter) VimClient() *vim25.Client {
	return dc.Client.VimClient()
}

// NewFinder create a finder
func (dc *Datacenter) NewFinder(ctx *Context) *find.Finder {
	d := dc.Datacenter(ctx)

	f := find.NewFinder(dc.VimClient(), true)
	f.SetDatacenter(d)

	return f
}

// GetDatastore retrieve named datastore
func (dc *Datacenter) GetDatastore(ctx *Context, name string) (*Datastore, error) {
	var ds *object.Datastore
	var err error

	f := dc.NewFinder(ctx)

	if ds, err = f.Datastore(ctx, name); err == nil {
		return &Datastore{
			Ref:        ds.Reference(),
			Datacenter: dc,
		}, nil
	}

	return nil, err
}
