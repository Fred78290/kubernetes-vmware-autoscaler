package types

import "github.com/Fred78290/kubernetes-vmware-autoscaler/vsphere"

// ResourceLimiter define limit, not really used
type ResourceLimiter struct {
	MinLimits map[string]int64 `json:"min"`
	MaxLimits map[string]int64 `json:"max"`
}

// MachineCharacteristic defines VM kind
type MachineCharacteristic struct {
	Memory int `json:"memsize"`  // VM Memory size in megabytes
	Vcpu   int `json:"vcpus"`    // VM number of cpus
	Disk   int `json:"disksize"` // VM disk size in megabytes
}

// KubeJoinConfig give element to join kube master
type KubeJoinConfig struct {
	Address        string   `json:"address,omitempty"`
	Token          string   `json:"token,omitempty"`
	CACert         string   `json:"ca,omitempty"`
	ExtraArguments []string `json:"extras-args,omitempty"`
}

// AutoScalerServerOptionals declare wich features must be optional
type AutoScalerServerOptionals struct {
	Pricing                  bool `json:"pricing"`
	GetAvailableMachineTypes bool `json:"getAvailableMachineTypes"`
	NewNodeGroup             bool `json:"newNodeGroup"`
	TemplateNodeInfo         bool `json:"templateNodeInfo"`
	Create                   bool `json:"create"`
	Delete                   bool `json:"delete"`
}

// AutoScalerServerSSH contains ssh client infos
type AutoScalerServerSSH struct {
	UserName string `json:"user"`
	Password string `json:"password"`
	AuthKeys string `json:"ssh-key"`
}

// AutoScalerServerRsync declare an rsync operation
type AutoScalerServerRsync struct {
	Source      string   `json:"source"`
	Destination string   `json:"destination"`
	Excludes    []string `json:"excludes"`
}

// AutoScalerServerSyncFolders declare how to sync file between host and guest
type AutoScalerServerSyncFolders struct {
	RsyncOptions []string                `json:"options"`
	RsyncUser    string                  `json:"user"`
	RsyncSSHKey  string                  `json:"ssh-key"`
	Folders      []AutoScalerServerRsync `json:"folders"`
}

// AutoScalerServerConfig is contains configuration
type AutoScalerServerConfig struct {
	Network            string                            `default:"tcp" json:"network"`         // Mandatory, Network to listen (see grpc doc) to listen
	Listen             string                            `default:"0.0.0.0:5200" json:"listen"` // Mandatory, Address to listen
	ProviderID         string                            `json:"secret"`                        // Mandatory, secret Identifier, client must match this
	MinNode            int                               `json:"minNode"`                       // Mandatory, Min AutoScaler VM
	MaxNode            int                               `json:"maxNode"`                       // Mandatory, Max AutoScaler VM
	NodePrice          float64                           `json:"nodePrice"`                     // Optional, The VM price
	PodPrice           float64                           `json:"podPrice"`                      // Optional, The pod price
	KubeCtlConfig      string                            `default:"/etc/kubernetes/config" json:"kubeconfig"`
	KubeAdm            KubeJoinConfig                    `json:"kubeadm"`
	DefaultMachineType string                            `default:"{\"standard\": {}}" json:"default-machine"`
	Machines           map[string]*MachineCharacteristic `default:"{\"standard\": {}}" json:"machines"` // Mandatory, Available machines
	CloudInit          interface{}                       `json:"cloud-init"`                            // Optional, The cloud init conf file
	SyncFolders        *AutoScalerServerSyncFolders      `json:"sync-folder"`                           // Optional, do rsync between host and guest
	Optionals          *AutoScalerServerOptionals        `json:"optionals"`
	SSH                *AutoScalerServerSSH              `json:"ssh-infos"`
	VMwareInfos        map[string]*vsphere.Configuration `json:"vsphere"`
}
