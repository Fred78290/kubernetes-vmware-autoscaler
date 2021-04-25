package vsphere

import (
	"github.com/Fred78290/kubernetes-vmware-autoscaler/context"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/types"
)

var datacenterKey = contextKey("datacenter")

// Datacenter represent a datacenter
type Datacenter struct {
	Ref    types.ManagedObjectReference
	Name   string
	Client *Client
}

// Datacenter return a Datacenter
func (dc *Datacenter) Datacenter(ctx *context.Context) *object.Datacenter {
	if d := ctx.Value(datacenterKey); d != nil {
		return d.(*object.Datacenter)
	}

	f := find.NewFinder(dc.VimClient(), true)

	d, err := f.Datacenter(ctx, dc.Name)

	if err != nil {
		glog.Fatalf("Can't find datacenter:%s", dc.Name)
	}

	ctx.WithValue(datacenterKey, d)

	return d
}

// VimClient return the VIM25 client
func (dc *Datacenter) VimClient() *vim25.Client {
	return dc.Client.VimClient()
}

// NewFinder create a finder
func (dc *Datacenter) NewFinder(ctx *context.Context) *find.Finder {
	d := object.NewDatacenter(dc.VimClient(), dc.Ref)
	f := find.NewFinder(dc.VimClient(), true)
	f.SetDatacenter(d)

	return f
}

// GetDatastore retrieve named datastore
func (dc *Datacenter) GetDatastore(ctx *context.Context, name string) (*Datastore, error) {
	var ds *object.Datastore
	var err error

	f := dc.NewFinder(ctx)

	if ds, err = f.Datastore(ctx, name); err == nil {
		return &Datastore{
			Ref:        ds.Reference(),
			Name:       name,
			Datacenter: dc,
		}, nil
	}

	return nil, err
}
