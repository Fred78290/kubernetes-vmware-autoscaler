package vsphere

import (
	"github.com/Fred78290/kubernetes-vmware-autoscaler/context"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/types"
)

type HostAutoStartManager struct {
	Ref        types.ManagedObjectReference
	Datacenter *Datacenter
}

func (h *HostAutoStartManager) SetAutoStart(ctx *context.Context, datastore, name string, startOrder int) error {
	var err error
	var ds *Datastore
	var vm *VirtualMachine

	dc := h.Datacenter

	if ds, err = dc.GetDatastore(ctx, datastore); err == nil {
		if vm, err = ds.VirtualMachine(ctx, name); err == nil {
			powerInfo := []types.AutoStartPowerInfo{{
				Key:              vm.Ref,
				StartOrder:       int32(startOrder),
				StartDelay:       -1,
				WaitForHeartbeat: types.AutoStartWaitHeartbeatSettingSystemDefault,
				StartAction:      "powerOn",
				StopDelay:        -1,
				StopAction:       "systemDefault",
			}}

			req := types.ReconfigureAutostart{
				This: h.Ref,
				Spec: types.HostAutoStartManagerConfig{
					PowerInfo: powerInfo,
				},
			}

			_, err = methods.ReconfigureAutostart(ctx, dc.VimClient(), &req)
		}
	}

	return err
}
