package vsphere

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/golang/glog"

	"github.com/vmware/govmomi/govc/flags"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/types"
)

// Key type for storing flag instances in a context.Context.
type contextKey string

var outputKey = contextKey("output")
var datastoreKey = contextKey("datastore")

// Datastore datastore wrapper
type Datastore struct {
	Ref        types.ManagedObjectReference
	Name       string
	Datacenter *Datacenter
}

type listOutput struct {
	rs      []types.HostDatastoreBrowserSearchResults
	recurse bool
	all     bool
	slash   bool
}

func (o *listOutput) add(r types.HostDatastoreBrowserSearchResults) {
	if o.recurse && !o.all {
		// filter out ".hidden" directories
		path := strings.SplitN(r.FolderPath, " ", 2)
		if len(path) == 2 {
			path = strings.Split(path[1], "/")
			if path[0] == "." {
				path = path[1:]
			}

			for _, p := range path {
				if len(p) != 0 && p[0] == '.' {
					return
				}
			}
		}
	}

	res := r
	res.File = nil

	for _, f := range r.File {
		if f.GetFileInfo().Path[0] == '.' && !o.all {
			continue
		}

		if o.slash {
			if d, ok := f.(*types.FolderFileInfo); ok {
				d.Path += "/"
			}
		}

		res.File = append(res.File, f)
	}

	o.rs = append(o.rs, res)
}

// Datastore create govmomi Datastore object
func (ds *Datastore) Datastore(ctx *Context) *object.Datastore {
	if d := ctx.Value(datastoreKey); d != nil {
		return d.(*object.Datastore)
	}

	f := ds.Datacenter.NewFinder(ctx)

	d, err := f.ObjectReference(ctx, ds.Ref)

	if err != nil {
		glog.Fatalf("Can't find datastore:%s", ds.Name)
	}
	//	d := object.NewDatastore(ds.Datacenter.VimClient(), ds.Ref)

	ctx.WithValue(datastoreKey, d)

	return d.(*object.Datastore)
}

// VimClient return the VIM25 client
func (ds *Datastore) VimClient() *vim25.Client {
	return ds.Datacenter.VimClient()
}

func (ds *Datastore) findVM(ctx *Context, name string) (*object.VirtualMachine, error) {
	key := fmt.Sprintf("[%s] %s", ds.Name, name)

	if v := ctx.Value(key); v != nil {
		return v.(*object.VirtualMachine), nil
	}

	f := ds.Datacenter.NewFinder(ctx)

	vm, err := f.VirtualMachine(ctx, name)

	if err == nil {
		ctx.WithValue(key, vm)
		ctx.WithValue(vm.Reference().String(), vm)
	}

	return vm, err
}

func (ds *Datastore) resourcePool(ctx *Context, name string) (*object.ResourcePool, error) {
	f := ds.Datacenter.NewFinder(ctx)

	return f.ResourcePoolOrDefault(ctx, name)
}

func (ds *Datastore) folder(ctx *Context, name string) (*object.Folder, error) {
	f := ds.Datacenter.NewFinder(ctx)

	return f.FolderOrDefault(ctx, name)
}

func (ds *Datastore) output(ctx *Context) *flags.OutputFlag {
	if v := ctx.Value(outputKey); v != nil {
		return v.(*flags.OutputFlag)
	}

	v := &flags.OutputFlag{Out: os.Stdout}
	ctx.WithValue(outputKey, v)

	return v
}

// CreateVirtualMachine create a new virtual machine
func (ds *Datastore) CreateVirtualMachine(ctx *Context, name string) (*VirtualMachine, error) {
	var templateVM *object.VirtualMachine
	var folder *object.Folder
	var resourcePool *object.ResourcePool
	var devices object.VirtualDeviceList
	var task *object.Task
	var err error
	var vm *VirtualMachine

	config := ds.Datacenter.Client.Configuration
	output := ds.output(ctx)

	if templateVM, err = ds.findVM(ctx, config.TemplateName); err == nil {

		logger := output.ProgressLogger(fmt.Sprintf("Cloning %s to %s...", templateVM.InventoryPath, name))
		defer logger.Wait()

		if folder, err = ds.folder(ctx, config.VMBasePath); err == nil {
			if resourcePool, err = ds.resourcePool(ctx, config.Resource); err == nil {
				if devices, err = templateVM.Device(ctx); err == nil {

					// prepare virtual device config spec for network card
					configSpecs := []types.BaseVirtualDeviceConfigSpec{}

					if config.Network != nil {
						op := types.VirtualDeviceConfigSpecOperationAdd
						card, derr := config.Network.Device(ds.VimClient(), ds.Datacenter.Datacenter(ctx))
						if derr != nil {
							return nil, derr
						}
						// search for the first network card of the source
						for _, device := range devices {
							if _, ok := device.(types.BaseVirtualEthernetCard); ok {
								op = types.VirtualDeviceConfigSpecOperationEdit
								// set new backing info
								config.Network.Change(device, card)
								card = device
								break
							}
						}

						configSpecs = append(configSpecs, &types.VirtualDeviceConfigSpec{
							Operation: op,
							Device:    card,
						})

						folderref := folder.Reference()
						poolref := resourcePool.Reference()

						cloneSpec := &types.VirtualMachineCloneSpec{
							PowerOn:  false,
							Template: config.Template,
						}

						relocateSpec := types.VirtualMachineRelocateSpec{
							DeviceChange: configSpecs,
							Folder:       &folderref,
							Pool:         &poolref,
						}

						if config.LinkedClone {
							relocateSpec.DiskMoveType = string(types.VirtualMachineRelocateDiskMoveOptionsMoveAllDiskBackingsAndAllowSharing)
						}

						cloneSpec.Location = relocateSpec
						cloneSpec.Location.Datastore = &ds.Ref

						// check if customization specification requested
						if len(config.Customization) > 0 {
							// get the customization spec manager
							customizationSpecManager := object.NewCustomizationSpecManager(ds.VimClient())
							// check if customization specification exists
							exists, err := customizationSpecManager.DoesCustomizationSpecExist(ctx, config.Customization)
							if err != nil {
								return nil, err
							}
							if exists == false {
								return nil, fmt.Errorf("Customization specification %s does not exists", config.Customization)
							}
							// get the customization specification
							customSpecItem, err := customizationSpecManager.GetCustomizationSpec(ctx, config.Customization)
							if err != nil {
								return nil, err
							}
							customSpec := customSpecItem.Spec
							// set the customization
							cloneSpec.Customization = &customSpec
						}

						if task, err = templateVM.Clone(ctx, folder, name, *cloneSpec); err == nil {
							var info *types.TaskInfo

							if info, err = task.WaitForResult(ctx, logger); err == nil {
								vm = &VirtualMachine{
									Ref:       info.Result.(types.ManagedObjectReference),
									Name:      name,
									Datastore: ds,
								}
							}
						}
					}
				}
			}
		}

	}

	return vm, err
}

// VirtualMachine retrieve the specified virtual machine
func (ds *Datastore) VirtualMachine(ctx *Context, name string) (*VirtualMachine, error) {

	vm, err := ds.findVM(ctx, name)

	if err == nil {
		return &VirtualMachine{
			Ref:       vm.Reference(),
			Name:      name,
			Datastore: ds,
		}, nil
	}

	return nil, err
}

// ListPath return object list matching path
func (ds *Datastore) ListPath(ctx *Context, b *object.HostDatastoreBrowser, path string, spec types.HostDatastoreBrowserSearchSpec, recurse bool) ([]types.HostDatastoreBrowserSearchResults, error) {
	path = ds.Datastore(ctx).Path(path)

	search := b.SearchDatastore
	if recurse {
		search = b.SearchDatastoreSubFolders
	}

	task, err := search(ctx, path, &spec)
	if err != nil {
		return nil, err
	}

	info, err := task.WaitForResult(ctx, nil)
	if err != nil {
		return nil, err
	}

	switch r := info.Result.(type) {
	case types.HostDatastoreBrowserSearchResults:
		return []types.HostDatastoreBrowserSearchResults{r}, nil
	case types.ArrayOfHostDatastoreBrowserSearchResults:
		return r.HostDatastoreBrowserSearchResults, nil
	default:
		panic(fmt.Sprintf("unknown result type: %T", r))
	}
}

func isInvalid(err error) bool {
	if f, ok := err.(types.HasFault); ok {
		switch f.Fault().(type) {
		case *types.InvalidArgument:
			return true
		}
	}

	return false
}

// List list VM inside the datastore
func (ds *Datastore) List(ctx *Context) ([]*VirtualMachine, error) {
	var browser *object.HostDatastoreBrowser
	var err error
	var vms []*VirtualMachine

	if browser, err = ds.Datastore(ctx).Browser(ctx); err == nil {

		spec := types.HostDatastoreBrowserSearchSpec{
			MatchPattern: []string{"*"},
		}

		fromPath := object.DatastorePath{
			Datastore: ds.Name,
			Path:      "",
		}

		arg := fromPath.Path

		result := &listOutput{
			rs:      make([]types.HostDatastoreBrowserSearchResults, 0),
			recurse: false,
			all:     false,
			slash:   true,
		}

		for i := 0; ; i++ {
			var r []types.HostDatastoreBrowserSearchResults

			r, err = ds.ListPath(ctx, browser, arg, spec, false)

			if err != nil {
				// Treat the argument as a match pattern if not found as directory
				if i == 0 && types.IsFileNotFound(err) || isInvalid(err) {
					spec.MatchPattern[0] = path.Base(arg)
					arg = path.Dir(arg)
					continue
				}

				return nil, err
			}

			for n := range r {
				result.add(r[n])
			}

			break
		}

		f := ds.Datacenter.NewFinder(ctx)

		vms = make([]*VirtualMachine, 0, len(result.rs))

		for _, item := range result.rs {
			// Find virtual machines in datacenter
			for _, file := range item.File {
				info := file.GetFileInfo()
				vm, err := f.VirtualMachine(ctx, info.Path)

				if err == nil {
					vms = append(vms, &VirtualMachine{
						Ref:       vm.Reference(),
						Name:      vm.Name(),
						Datastore: ds,
					})
				}
			}
		}
	}

	return vms, err
}
