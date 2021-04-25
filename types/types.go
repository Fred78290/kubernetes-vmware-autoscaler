package types

import (
	"fmt"
	"os/user"
	"strconv"
	"strings"
	"time"

	"github.com/Fred78290/kubernetes-vmware-autoscaler/constantes"
	"github.com/Fred78290/kubernetes-vmware-autoscaler/vsphere"
	"github.com/alecthomas/kingpin"
	glog "github.com/sirupsen/logrus"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	DefaultMaxGracePeriod    time.Duration = 30 * time.Second
	DefaultMaxRequestTimeout time.Duration = 30 * time.Second
	DefaultMaxDeletionPeriod time.Duration = 2 * time.Minute
)

type Config struct {
	APIServerURL    string
	KubeConfig      string
	RequestTimeout  time.Duration
	DeletionTimeout time.Duration
	MaxGracePeriod  time.Duration
	Config          string
	SaveLocation    string
	DisplayVersion  bool
	LogFormat       string
	LogLevel        string
	MinCpus         int64
	MinMemory       int64
	MaxCpus         int64
	MaxMemory       int64
}

func (c *Config) GetResourceLimiter() *ResourceLimiter {
	return &ResourceLimiter{
		MinLimits: map[string]int64{constantes.ResourceNameCores: c.MinCpus, constantes.ResourceNameMemory: c.MinMemory * 1024 * 1024},
		MaxLimits: map[string]int64{constantes.ResourceNameCores: c.MaxCpus, constantes.ResourceNameMemory: c.MaxMemory * 1024 * 1024},
	}
}

// A PodFilterFunc returns true if the supplied pod passes the filter.
type PodFilterFunc func(p apiv1.Pod) (bool, error)

// ClientGenerator provides clients
type ClientGenerator interface {
	KubeClient() (kubernetes.Interface, error)

	PodList(nodeName string, podFilter PodFilterFunc) ([]apiv1.Pod, error)
	NodeList() (*apiv1.NodeList, error)
	UncordonNode(nodeName string) error
	CordonNode(nodeName string) error
	MarkDrainNode(nodeName string) error
	DrainNode(nodeName string, ignoreDaemonSet, deleteLocalData bool) error
	DeleteNode(nodeName string) error
	AnnoteNode(nodeName string, annotations map[string]string) error
	LabelNode(nodeName string, labels map[string]string) error
	WaitNodeToBeReady(nodeName string, timeToWaitInSeconds int) error
}

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
	AuthKeys string `json:"ssh-private-key"`
}

// GetUserName returns user name from config or the real current username is empty or equal to ~
func (ssh *AutoScalerServerSSH) GetUserName() string {
	if ssh.UserName == "" || ssh.UserName == "~" {
		u, err := user.Current()

		if err != nil {
			glog.Fatalf("Can't find current user! - %v", err)
		}

		return u.Username
	}

	return ssh.UserName
}

// GetAuthKeys returns the path to key file, subsistute ~
func (ssh *AutoScalerServerSSH) GetAuthKeys() string {
	if strings.Index(ssh.AuthKeys, "~") == 0 {
		u, err := user.Current()

		if err != nil {
			glog.Fatalf("Can't find current user! - %v", err)
		}

		return strings.Replace(ssh.AuthKeys, "~", u.HomeDir, 1)
	}

	return ssh.AuthKeys
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
	KubeAdm            KubeJoinConfig                    `json:"kubeadm"`
	DefaultMachineType string                            `default:"{\"standard\": {}}" json:"default-machine"`
	Machines           map[string]*MachineCharacteristic `default:"{\"standard\": {}}" json:"machines"` // Mandatory, Available machines
	CloudInit          interface{}                       `json:"cloud-init"`                            // Optional, The cloud init conf file
	SyncFolders        *AutoScalerServerSyncFolders      `json:"sync-folder"`                           // Optional, do rsync between host and guest
	Optionals          *AutoScalerServerOptionals        `json:"optionals"`
	ResourceLimiter    *ResourceLimiter                  `json:"limits"`
	SSH                *AutoScalerServerSSH              `json:"ssh-infos"`
	VMwareInfos        map[string]*vsphere.Configuration `json:"vmware"`
}

// GetVSphereConfiguration returns the vsphere named conf or default
func (conf *AutoScalerServerConfig) GetVSphereConfiguration(name string) *vsphere.Configuration {
	var vsphere *vsphere.Configuration

	if vsphere = conf.VMwareInfos[name]; vsphere == nil {
		vsphere = conf.VMwareInfos["default"]
	}

	if vsphere == nil {
		glog.Fatalf("Unable to find vmware config for name:%s", name)
	}

	return vsphere
}

// NewConfig returns new Config object
func NewConfig() *Config {
	return &Config{
		APIServerURL:    "",
		KubeConfig:      "",
		RequestTimeout:  DefaultMaxRequestTimeout,
		DeletionTimeout: DefaultMaxDeletionPeriod,
		MaxGracePeriod:  DefaultMaxGracePeriod,
		DisplayVersion:  false,
		Config:          "/etc/cluster/vmware-cluster-autoscaler.json",
		MinCpus:         2,
		MinMemory:       1024,
		MaxCpus:         24,
		MaxMemory:       1024 * 24,
		LogFormat:       "text",
		LogLevel:        glog.InfoLevel.String(),
	}
}

// allLogLevelsAsStrings returns all logrus levels as a list of strings
func allLogLevelsAsStrings() []string {
	var levels []string
	for _, level := range glog.AllLevels {
		levels = append(levels, level.String())
	}
	return levels
}

func (cfg *Config) ParseFlags(args []string, version string) error {
	app := kingpin.New("vmware-autoscaler", "Kubernetes VMWare autoscaler create VM instances at demand for autoscaling.\n\nNote that all flags may be replaced with env vars - `--flag` -> `VMWARE_AUTOSCALER_FLAG=1` or `--flag value` -> `VMWARE_AUTOSCALER_FLAG=value`")

	//	app.Version(version)
	app.HelpFlag.Short('h')
	app.DefaultEnvars()

	app.Flag("log-format", "The format in which log messages are printed (default: text, options: text, json)").Default(cfg.LogFormat).EnumVar(&cfg.LogFormat, "text", "json")
	app.Flag("log-level", "Set the level of logging. (default: info, options: panic, debug, info, warning, error, fatal").Default(cfg.LogLevel).EnumVar(&cfg.LogLevel, allLogLevelsAsStrings()...)

	// Flags related to Kubernetes
	app.Flag("server", "The Kubernetes API server to connect to (default: auto-detect)").Default(cfg.APIServerURL).StringVar(&cfg.APIServerURL)
	app.Flag("kubeconfig", "Retrieve target cluster configuration from a Kubernetes configuration file (default: auto-detect)").Default(cfg.KubeConfig).StringVar(&cfg.KubeConfig)
	app.Flag("request-timeout", "Request timeout when calling Kubernetes APIs. 0s means no timeout").Default(DefaultMaxRequestTimeout.String()).DurationVar(&cfg.RequestTimeout)
	app.Flag("deletion-timeout", "Deletion timeout when delete node. 0s means no timeout").Default(DefaultMaxDeletionPeriod.String()).DurationVar(&cfg.DeletionTimeout)
	app.Flag("max-grace-period", "Maximum time evicted pods will be given to terminate gracefully.").Default(DefaultMaxGracePeriod.String()).DurationVar(&cfg.MaxGracePeriod)

	app.Flag("min-cpus", "Limits: minimum cpu (default: 1)").Default(strconv.FormatInt(cfg.MinCpus, 10)).Int64Var(&cfg.MinCpus)
	app.Flag("min-memory", "Limits: minimum memory in MB (default: 1G)").Default(strconv.FormatInt(cfg.MinMemory, 10)).Int64Var(&cfg.MinMemory)
	app.Flag("max-cpus", "Limits: max cpu (default: 24)").Default(strconv.FormatInt(cfg.MaxCpus, 10)).Int64Var(&cfg.MaxCpus)
	app.Flag("max-memory", "Limits: max memory in MB (default: 24G)").Default(strconv.FormatInt(cfg.MaxMemory, 10)).Int64Var(&cfg.MaxMemory)

	app.Flag("version", "Display version and exit").BoolVar(&cfg.DisplayVersion)

	app.Flag("config", "The config for the server").Default(cfg.Config).StringVar(&cfg.Config)
	app.Flag("save", "The file to persists the server").Default(cfg.SaveLocation).StringVar(&cfg.SaveLocation)

	_, err := app.Parse(args)
	if err != nil {
		return err
	}

	return nil
}

func (cfg *Config) String() string {
	return fmt.Sprintf("APIServerURL:%s KubeConfig:%s RequestTimeout:%s Config:%s SaveLocation:%s DisplayVersion:%s", cfg.APIServerURL, cfg.KubeConfig, cfg.RequestTimeout, cfg.Config, cfg.SaveLocation, strconv.FormatBool(cfg.DisplayVersion))
}
