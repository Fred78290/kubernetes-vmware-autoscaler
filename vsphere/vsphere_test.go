package vsphere_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/golang/glog"
	"github.com/stretchr/testify/assert"

	"github.com/Fred78290/kubernetes-vmware-autoscaler/types"
	"github.com/Fred78290/kubernetes-vmware-autoscaler/vsphere"
)

type ConfigurationTest struct {
	vsphere.Configuration
	CloudInit interface{}               `json:"cloud-init"`
	SSH       types.AutoScalerServerSSH `json:"ssh"`
	VM        string                    `json:"old-vm"`
	New       *NewVirtualMachineConf    `json:"new-vm"`
}

type NewVirtualMachineConf struct {
	Name       string
	Annotation string
	Memory     int
	CPUS       int
	Disk       int
	Network    *vsphere.Network
}

var testConfig *ConfigurationTest
var confName = "agora.json"

func saveToJson(fileName string, config *ConfigurationTest) error {
	file, err := os.Create(fileName)

	if err != nil {
		glog.Errorf("Failed to open file:%s, error:%v", fileName, err)

		return err
	}

	defer file.Close()

	encoder := json.NewEncoder(file)
	err = encoder.Encode(config)

	if err != nil {
		glog.Errorf("failed to encode config to file:%s, error:%v", fileName, err)

		return err
	}

	return nil
}

func loadFromJson(fileName string) *ConfigurationTest {
	if testConfig == nil {
		file, err := os.Open(fileName)
		if err != nil {
			glog.Fatalf("failed to open config file:%s, error:%v", fileName, err)
		}

		decoder := json.NewDecoder(file)
		err = decoder.Decode(&testConfig)
		if err != nil {
			glog.Fatalf("failed to decode config file:%s, error:%v", fileName, err)
		}
	}

	return testConfig
}

func Test_getVM(t *testing.T) {
	config := loadFromJson(confName)

	ctx := vsphere.NewContext(config.Timeout)
	defer ctx.Cancel()

	vm, err := config.VirtualMachineWithContext(ctx, config.VM)

	if assert.NoError(t, err) {
		if assert.NotNil(t, vm) {
			status, err := vm.Status(ctx)

			if assert.NoErrorf(t, err, "Can't get status of VM") {
				t.Logf("The power of vm is:%v", status.Powered)
			}
		}
	}
}

func Test_listVM(t *testing.T) {
	config := loadFromJson(confName)

	ctx := vsphere.NewContext(config.Timeout)
	defer ctx.Cancel()

	vms, err := config.VirtualMachineListWithContext(ctx)

	if assert.NoError(t, err) {
		if assert.NotNil(t, vms) {
			for _, vm := range vms {
				status, err := vm.Status(ctx)

				if assert.NoErrorf(t, err, "Can't get status of VM") {
					t.Logf("The power of vm %s is:%v", vm.Name, status.Powered)
				}
			}
		}
	}
}

func Test_createVM(t *testing.T) {
	config := loadFromJson(confName)

	_, err := config.Create(config.New.Name, config.SSH.UserName, config.SSH.AuthKeys, config.CloudInit, config.New.Network, config.New.Annotation, config.New.Memory, config.New.CPUS, config.New.Disk)

	if assert.NoError(t, err, "Can't create VM") {
		t.Logf("VM created")
	}
}

func Test_statusVM(t *testing.T) {
	config := loadFromJson(confName)

	status, err := config.Status(config.New.Name)

	if assert.NoError(t, err, "Can't get status VM") {
		t.Logf("The power of vm %s is:%v", config.New.Name, status.Powered)
	}
}

func Test_powerOnVM(t *testing.T) {
	config := loadFromJson(confName)

	err := config.PowerOn(config.New.Name)

	if assert.NoError(t, err, "Can't power on VM") {
		ipaddr, err := config.WaitForIP(config.New.Name)

		if assert.NoError(t, err, "Can't get IP") {
			t.Logf("VM powered with IP:%s", ipaddr)
		}
	}
}

func Test_powerOffVM(t *testing.T) {
	config := loadFromJson(confName)

	err := config.PowerOff(config.New.Name)

	if assert.NoError(t, err, "Can't power off VM") {
		t.Logf("VM shutdown")
	}
}

func Test_deleteVM(t *testing.T) {
	config := loadFromJson(confName)

	err := config.Delete(config.New.Name)

	if assert.NoError(t, err, "Can't delete VM") {
		t.Logf("VM deleted")
	}
}
