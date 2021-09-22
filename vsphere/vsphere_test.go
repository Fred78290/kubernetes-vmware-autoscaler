package vsphere_test

import (
	"encoding/json"
	"os"
	"testing"

	glog "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/Fred78290/kubernetes-vmware-autoscaler/context"
	"github.com/Fred78290/kubernetes-vmware-autoscaler/types"
	"github.com/Fred78290/kubernetes-vmware-autoscaler/utils"
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
var confName = "test.json"

func testFeature(name string) bool {
	if feature := os.Getenv(name); feature != "" {
		return feature != "NO"
	}

	return true
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

func Test_AuthMethodKey(t *testing.T) {
	if testFeature("Test_AuthMethodKey") {
		config := loadFromJson(confName)

		signer := utils.AuthMethodFromPrivateKeyFile(config.SSH.GetAuthKeys())

		_ = assert.NotNil(t, signer)
	}
}

func Test_Sudo(t *testing.T) {
	if testFeature("Test_Sudo") {
		config := loadFromJson(confName)

		out, err := utils.Sudo(&config.SSH, "localhost", "ls")

		if assert.NoError(t, err) {
			t.Log(out)
		}
	}
}

func Test_CIDR(t *testing.T) {
	if testFeature("Test_CIDR") {

		cidr := vsphere.ToCIDR("10.65.4.201", "255.255.255.0")

		if assert.Equal(t, cidr, "10.65.4.201/24") {
			cidr := vsphere.ToCIDR("10.65.4.201", "")
			assert.Equal(t, cidr, "10.65.4.201/8")
		}
	}
}

func Test_getVM(t *testing.T) {
	if testFeature("Test_getVM") {
		config := loadFromJson(confName)

		ctx := context.NewContext(config.Timeout)
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
}

func Test_listVM(t *testing.T) {
	if testFeature("Test_listVM") {
		config := loadFromJson(confName)

		ctx := context.NewContext(config.Timeout)
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
}

func Test_createVM(t *testing.T) {
	if testFeature("Test_createVM") {
		config := loadFromJson(confName)

		_, err := config.Create(config.New.Name, config.SSH.GetUserName(), config.SSH.GetAuthKeys(), config.CloudInit, config.New.Network, config.New.Annotation, config.New.Memory, config.New.CPUS, config.New.Disk, 0)

		if assert.NoError(t, err, "Can't create VM") {
			t.Logf("VM created")
		}
	}
}

func Test_statusVM(t *testing.T) {
	if testFeature("Test_statusVM") {
		config := loadFromJson(confName)

		status, err := config.Status(config.New.Name)

		if assert.NoError(t, err, "Can't get status VM") {
			t.Logf("The power of vm %s is:%v", config.New.Name, status.Powered)
		}
	}
}

func Test_powerOnVM(t *testing.T) {
	if testFeature("Test_powerOnVM") {
		config := loadFromJson(confName)

		if status, err := config.Status(config.New.Name); assert.NoError(t, err, "Can't get status on VM") && status.Powered == false {
			err = config.PowerOn(config.New.Name)

			if assert.NoError(t, err, "Can't power on VM") {
				ipaddr, err := config.WaitForIP(config.New.Name)

				if assert.NoError(t, err, "Can't get IP") {
					t.Logf("VM powered with IP:%s", ipaddr)
				}
			}
		}
	}
}

func Test_powerOffVM(t *testing.T) {
	if testFeature("Test_powerOffVM") {
		config := loadFromJson(confName)

		if status, err := config.Status(config.New.Name); assert.NoError(t, err, "Can't get status on VM") && status.Powered {
			err = config.PowerOff(config.New.Name)

			if assert.NoError(t, err, "Can't power off VM") {
				t.Logf("VM shutdown")
			}
		}
	}
}

func Test_shutdownGuest(t *testing.T) {
	if testFeature("Test_shutdownGuest") {
		config := loadFromJson(confName)

		if status, err := config.Status(config.New.Name); assert.NoError(t, err, "Can't get status on VM") && status.Powered {
			err = config.ShutdownGuest(config.New.Name)

			if assert.NoError(t, err, "Can't power off VM") {
				t.Logf("VM shutdown")
			}
		}
	}
}

func Test_deleteVM(t *testing.T) {
	if testFeature("Test_deleteVM") {
		config := loadFromJson(confName)

		err := config.Delete(config.New.Name)

		if assert.NoError(t, err, "Can't delete VM") {
			t.Logf("VM deleted")
		}
	}
}
