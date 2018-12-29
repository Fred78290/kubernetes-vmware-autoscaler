package vsphere_test

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/golang/glog"
	"github.com/stretchr/testify/assert"

	"github.com/Fred78290/kubernetes-vmware-autoscaler/vsphere"
)

var testConfig = &vsphere.Configuration{
	Host:          "127.0.0.1:8989",
	UserName:      "user",
	Password:      "pass",
	Insecure:      true,
	DataCenter:    "DC0",
	DataStore:     "LocalDS_0",
	VMBasePath:    "",
	Timeout:       time.Hour * 1, // Allows debugging :)
	TemplateName:  "DC0_H0_VM0",
	Template:      false,
	LinkedClone:   true,
	Customization: "",
	Network: &vsphere.Network{
		Name:    "DC0_DVPG0",
		Adapter: "E1000",
	},
}

func saveToJson(fileName string, config *vsphere.Configuration) error {
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

func loadFromJson(fileName string) vsphere.Configuration {
	var config vsphere.Configuration

	file, err := os.Open(fileName)
	if err != nil {
		glog.Fatalf("failed to open config file:%s, error:%v", fileName, err)
	}

	decoder := json.NewDecoder(file)
	err = decoder.Decode(&config)
	if err != nil {
		glog.Fatalf("failed to decode config file:%s, error:%v", fileName, err)
	}

	return config
}

func Test_getVM(t *testing.T) {
	//	saveToJson("test.json", phConfig)

	config := loadFromJson("agora.json")

	ctx := vsphere.NewContext(config.Timeout)
	defer ctx.Cancel()

	vm, err := config.VirtualMachine(ctx, "DC0_H0_VM0")

	if assert.NoError(t, err) {
		if assert.NotNil(t, vm) {
			status, err := vm.Status(ctx)

			if assert.NoErrorf(t, err, "Can't get status of VM") {
				t.Logf("The power of vm is:%v", status.Powered)
			}
		}
	}
}
func Test_createVM(t *testing.T) {

}
func Test_launchVM(t *testing.T) {

}
