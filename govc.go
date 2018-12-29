package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/vmware/govmomi/govc/flags"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// VSphereInfos VSphereInfos
type vsphereInfos struct {
	*flags.ClientFlag
	*flags.SearchFlag
	vmName string
}

type cloneInfos struct {
	*flags.ClientFlag
	*flags.DatacenterFlag
	*flags.DatastoreFlag
	*flags.StoragePodFlag
	*flags.ResourcePoolFlag
	*flags.HostSystemFlag
	*flags.NetworkFlag
	*flags.FolderFlag
	*flags.VirtualMachineFlag

	name          string
	template      bool
	customization string
	link          bool

	Client         *vim25.Client
	Datacenter     *object.Datacenter
	Datastore      *object.Datastore
	StoragePod     *object.StoragePod
	ResourcePool   *object.ResourcePool
	HostSystem     *object.HostSystem
	Folder         *object.Folder
	VirtualMachine *object.VirtualMachine
}

type infoResult struct {
	VirtualMachines []mo.VirtualMachine
	objects         *object.VirtualMachine
	entities        map[types.ManagedObjectReference]string
}

func newVSphereInfos(vm string) *vsphereInfos {
	result := &vsphereInfos{
		vmName: vm,
	}

	return result
}

func (vm *vsphereInfos) register(ctx context.Context, f *flag.FlagSet) {
	vm.ClientFlag, ctx = flags.NewClientFlag(ctx)
	vm.ClientFlag.Register(ctx, f)

	vm.SearchFlag, ctx = flags.NewSearchFlag(ctx, flags.SearchVirtualMachines)
	vm.SearchFlag.Register(ctx, f)
}

func (vm *vsphereInfos) process(ctx context.Context) error {
	if err := vm.ClientFlag.Process(ctx); err != nil {
		return err
	}

	if err := vm.SearchFlag.Process(ctx); err != nil {
		return err
	}

	return nil
}

func (vm *vsphereInfos) buildFlags(f *flag.FlagSet, args ...string) {
	f.Parse(args)
}

func (vm *vsphereInfos) create(templateName string, memory int, cpus int, disk int) error {
	f := flag.NewFlagSet("delete", flag.ContinueOnError)
	ctx, cancel := context.WithTimeout(context.Background(), phAutoScalerServer.Configuration.VSphere.Timeout)
	defer cancel()
	clone := cloneInfos{
		name:     vm.vmName,
		link:     true,
		template: false,
	}

	clone.register(ctx, f)

	if err := clone.process(ctx); err != nil {
		return err
	}

	return clone.clone(ctx, memory, cpus)
}

func (vm *vsphereInfos) delete() error {
	f := flag.NewFlagSet("delete", flag.ContinueOnError)
	ctx, cancel := context.WithTimeout(context.Background(), phAutoScalerServer.Configuration.VSphere.Timeout)
	defer cancel()

	vm.register(ctx, f)

	if err := vm.process(ctx); err != nil {
		return err
	}

	vms, err := vm.VirtualMachines(f.Args())
	if err != nil {
		return err
	}

	for _, vm := range vms {
		task, err := vm.PowerOff(ctx)
		if err != nil {
			return err
		}

		// Ignore error since the VM may already been in powered off state.
		// vm.Destroy will fail if the VM is still powered on.
		_ = task.Wait(ctx)

		task, err = vm.Destroy(ctx)
		if err != nil {
			return err
		}

		err = task.Wait(ctx)
		if err != nil {
			return err
		}
	}

	return nil
}

func (vm *vsphereInfos) powerOn() error {
	f := flag.NewFlagSet("powerOn", flag.ContinueOnError)
	ctx, cancel := context.WithTimeout(context.Background(), phAutoScalerServer.Configuration.VSphere.Timeout)
	defer cancel()

	vm.register(ctx, f)

	if err := vm.process(ctx); err != nil {
		return err
	}

	vms, err := vm.VirtualMachines(f.Args())
	if err != nil {
		return err
	}

	for _, vm := range vms {
		task, err := vm.PowerOn(ctx)

		if err != nil {
			return err
		}

		err = task.Wait(ctx)

		if err != nil {
			return err
		}
	}

	return nil
}

func (vm *vsphereInfos) powerOff() error {
	f := flag.NewFlagSet("powerOff", flag.ContinueOnError)
	ctx, cancel := context.WithTimeout(context.Background(), phAutoScalerServer.Configuration.VSphere.Timeout)
	defer cancel()

	vm.register(ctx, f)

	if err := vm.process(ctx); err != nil {
		return err
	}

	vms, err := vm.VirtualMachines(f.Args())
	if err != nil {
		return err
	}

	for _, vm := range vms {
		task, err := vm.PowerOff(ctx)

		if err != nil {
			return err
		}

		err = task.Wait(ctx)

		if err != nil {
			return err
		}
	}

	return nil
}

func (vm *vsphereInfos) status() (*infoResult, error) {
	f := flag.NewFlagSet("status", flag.ContinueOnError)
	ctx, cancel := context.WithTimeout(context.Background(), phAutoScalerServer.Configuration.VSphere.Timeout)
	defer cancel()

	vm.register(ctx, f)

	if err := vm.process(ctx); err != nil {
		return nil, err
	}

	c, err := vm.Client()
	if err != nil {
		return nil, err
	}

	vms, err := vm.VirtualMachines(f.Args())
	if err != nil {
		return nil, err
	}

	refs := make([]types.ManagedObjectReference, 0, len(vms))
	for _, vm := range vms {
		refs = append(refs, vm.Reference())
	}

	var res infoResult

	pc := property.DefaultCollector(c)
	if len(refs) != 0 {
		err = pc.Retrieve(ctx, refs, nil, &res.VirtualMachines)
		if err != nil {
			return nil, err
		}
	}

	for i, vm := range res.VirtualMachines {
		if vm.Guest == nil || vm.Guest.IpAddress == "" {
			_, err = vms[i].WaitForIP(ctx)
			if err != nil {
				return nil, err
			}
			// Reload virtual machine object
			err = pc.RetrieveOne(ctx, vms[i].Reference(), nil, &res.VirtualMachines[i])
			if err != nil {
				return nil, err
			}
		}
	}

	return &res, nil
}

func (cmd *cloneInfos) register(ctx context.Context, f *flag.FlagSet) {
	cmd.ClientFlag, ctx = flags.NewClientFlag(ctx)
	cmd.ClientFlag.Register(ctx, f)

	cmd.DatacenterFlag, ctx = flags.NewDatacenterFlag(ctx)
	cmd.DatacenterFlag.Register(ctx, f)

	cmd.DatastoreFlag, ctx = flags.NewDatastoreFlag(ctx)
	cmd.DatastoreFlag.Register(ctx, f)

	cmd.StoragePodFlag, ctx = flags.NewStoragePodFlag(ctx)
	cmd.StoragePodFlag.Register(ctx, f)

	cmd.ResourcePoolFlag, ctx = flags.NewResourcePoolFlag(ctx)
	cmd.ResourcePoolFlag.Register(ctx, f)

	cmd.HostSystemFlag, ctx = flags.NewHostSystemFlag(ctx)
	cmd.HostSystemFlag.Register(ctx, f)

	cmd.NetworkFlag, ctx = flags.NewNetworkFlag(ctx)
	cmd.NetworkFlag.Register(ctx, f)

	cmd.FolderFlag, ctx = flags.NewFolderFlag(ctx)
	cmd.FolderFlag.Register(ctx, f)

	cmd.VirtualMachineFlag, ctx = flags.NewVirtualMachineFlag(ctx)
	cmd.VirtualMachineFlag.Register(ctx, f)
}

func (cmd *cloneInfos) process(ctx context.Context) error {
	if err := cmd.ClientFlag.Process(ctx); err != nil {
		return err
	}

	if err := cmd.DatacenterFlag.Process(ctx); err != nil {
		return err
	}

	if err := cmd.DatastoreFlag.Process(ctx); err != nil {
		return err
	}

	if err := cmd.StoragePodFlag.Process(ctx); err != nil {
		return err
	}

	if err := cmd.ResourcePoolFlag.Process(ctx); err != nil {
		return err
	}

	if err := cmd.HostSystemFlag.Process(ctx); err != nil {
		return err
	}

	if err := cmd.NetworkFlag.Process(ctx); err != nil {
		return err
	}

	if err := cmd.FolderFlag.Process(ctx); err != nil {
		return err
	}

	if err := cmd.VirtualMachineFlag.Process(ctx); err != nil {
		return err
	}

	return nil
}

func (cmd *cloneInfos) clone(ctx context.Context, memory int, cpus int) error {
	var err error

	cmd.Client, err = cmd.ClientFlag.Client()
	if err != nil {
		return err
	}

	cmd.Datacenter, err = cmd.DatacenterFlag.Datacenter()
	if err != nil {
		return err
	}

	if cmd.StoragePodFlag.Isset() {
		cmd.StoragePod, err = cmd.StoragePodFlag.StoragePod()
		if err != nil {
			return err
		}
	} else {
		cmd.Datastore, err = cmd.DatastoreFlag.Datastore()
		if err != nil {
			return err
		}
	}

	cmd.HostSystem, err = cmd.HostSystemFlag.HostSystemIfSpecified()
	if err != nil {
		return err
	}

	if cmd.HostSystem != nil {
		if cmd.ResourcePool, err = cmd.HostSystem.ResourcePool(ctx); err != nil {
			return err
		}
	} else {
		// -host is optional
		if cmd.ResourcePool, err = cmd.ResourcePoolFlag.ResourcePool(); err != nil {
			return err
		}
	}

	if cmd.Folder, err = cmd.FolderFlag.Folder(); err != nil {
		return err
	}

	if cmd.VirtualMachine, err = cmd.VirtualMachineFlag.VirtualMachine(); err != nil {
		return err
	}

	if cmd.VirtualMachine == nil {
		return flag.ErrHelp
	}

	vm, err := cmd.cloneVM(ctx)
	if err != nil {
		return err
	}

	if cpus > 0 || memory > 0 {
		vmConfigSpec := types.VirtualMachineConfigSpec{}

		if cpus > 0 {
			vmConfigSpec.NumCPUs = int32(cpus)
		}

		if memory > 0 {
			vmConfigSpec.MemoryMB = int64(memory)
		}

		task, err := vm.Reconfigure(ctx, vmConfigSpec)

		if err != nil {
			return err
		}

		_, err = task.WaitForResult(ctx, nil)

		if err != nil {
			return err
		}
	}

	task, err := vm.PowerOn(ctx)
	if err != nil {
		return err
	}

	_, err = task.WaitForResult(ctx, nil)
	if err != nil {
		return err
	}

	_, err = vm.WaitForIP(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (cmd *cloneInfos) cloneVM(ctx context.Context) (*object.VirtualMachine, error) {
	devices, err := cmd.VirtualMachine.Device(ctx)
	if err != nil {
		return nil, err
	}

	// prepare virtual device config spec for network card
	configSpecs := []types.BaseVirtualDeviceConfigSpec{}

	if cmd.NetworkFlag.IsSet() {
		op := types.VirtualDeviceConfigSpecOperationAdd
		card, derr := cmd.NetworkFlag.Device()
		if derr != nil {
			return nil, derr
		}
		// search for the first network card of the source
		for _, device := range devices {
			if _, ok := device.(types.BaseVirtualEthernetCard); ok {
				op = types.VirtualDeviceConfigSpecOperationEdit
				// set new backing info
				cmd.NetworkFlag.Change(device, card)
				card = device
				break
			}
		}

		configSpecs = append(configSpecs, &types.VirtualDeviceConfigSpec{
			Operation: op,
			Device:    card,
		})
	}

	folderref := cmd.Folder.Reference()
	poolref := cmd.ResourcePool.Reference()

	relocateSpec := types.VirtualMachineRelocateSpec{
		DeviceChange: configSpecs,
		Folder:       &folderref,
		Pool:         &poolref,
	}

	if cmd.HostSystem != nil {
		hostref := cmd.HostSystem.Reference()
		relocateSpec.Host = &hostref
	}

	cloneSpec := &types.VirtualMachineCloneSpec{
		PowerOn:  false,
		Template: cmd.template,
	}

	if cmd.link {
		relocateSpec.DiskMoveType = string(types.VirtualMachineRelocateDiskMoveOptionsMoveAllDiskBackingsAndAllowSharing)
	}

	cloneSpec.Location = relocateSpec

	// clone to storage pod
	datastoreref := types.ManagedObjectReference{}
	if cmd.StoragePod != nil && cmd.Datastore == nil {
		storagePod := cmd.StoragePod.Reference()

		// Build pod selection spec from config spec
		podSelectionSpec := types.StorageDrsPodSelectionSpec{
			StoragePod: &storagePod,
		}

		// Get the virtual machine reference
		vmref := cmd.VirtualMachine.Reference()

		// Build the placement spec
		storagePlacementSpec := types.StoragePlacementSpec{
			Folder:           &folderref,
			Vm:               &vmref,
			CloneName:        cmd.name,
			CloneSpec:        cloneSpec,
			PodSelectionSpec: podSelectionSpec,
			Type:             string(types.StoragePlacementSpecPlacementTypeClone),
		}

		// Get the storage placement result
		storageResourceManager := object.NewStorageResourceManager(cmd.Client)
		result, err := storageResourceManager.RecommendDatastores(ctx, storagePlacementSpec)
		if err != nil {
			return nil, err
		}

		// Get the recommendations
		recommendations := result.Recommendations
		if len(recommendations) == 0 {
			return nil, fmt.Errorf("no recommendations")
		}

		// Get the first recommendation
		datastoreref = recommendations[0].Action[0].(*types.StoragePlacementAction).Destination
	} else if cmd.StoragePod == nil && cmd.Datastore != nil {
		datastoreref = cmd.Datastore.Reference()
	} else {
		return nil, fmt.Errorf("Please provide either a datastore or a storagepod")
	}

	// Set the destination datastore
	cloneSpec.Location.Datastore = &datastoreref

	// Check if vmx already exists
	vmxPath := fmt.Sprintf("%s/%s.vmx", cmd.name, cmd.name)

	var mds mo.Datastore
	err = property.DefaultCollector(cmd.Client).RetrieveOne(ctx, datastoreref, []string{"name"}, &mds)
	if err != nil {
		return nil, err
	}

	datastore := object.NewDatastore(cmd.Client, datastoreref)
	datastore.InventoryPath = mds.Name

	_, err = datastore.Stat(ctx, vmxPath)
	if err == nil {
		dsPath := cmd.Datastore.Path(vmxPath)
		return nil, fmt.Errorf("File %s already exists", dsPath)
	}

	// check if customization specification requested
	if len(cmd.customization) > 0 {
		// get the customization spec manager
		customizationSpecManager := object.NewCustomizationSpecManager(cmd.Client)
		// check if customization specification exists
		exists, err := customizationSpecManager.DoesCustomizationSpecExist(ctx, cmd.customization)
		if err != nil {
			return nil, err
		}
		if exists == false {
			return nil, fmt.Errorf("Customization specification %s does not exists", cmd.customization)
		}
		// get the customization specification
		customSpecItem, err := customizationSpecManager.GetCustomizationSpec(ctx, cmd.customization)
		if err != nil {
			return nil, err
		}
		customSpec := customSpecItem.Spec
		// set the customization
		cloneSpec.Customization = &customSpec
	}

	task, err := cmd.VirtualMachine.Clone(ctx, cmd.Folder, cmd.name, *cloneSpec)
	if err != nil {
		return nil, err
	}

	logger := cmd.ProgressLogger(fmt.Sprintf("Cloning %s to %s...", cmd.VirtualMachine.InventoryPath, cmd.name))
	defer logger.Wait()

	info, err := task.WaitForResult(ctx, logger)
	if err != nil {
		return nil, err
	}

	return object.NewVirtualMachine(cmd.Client, info.Result.(types.ManagedObjectReference)), nil
}
