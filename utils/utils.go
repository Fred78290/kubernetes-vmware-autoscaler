package utils

import (
	"encoding/json"
	"os"
	"time"

	"github.com/Fred78290/kubernetes-vmware-autoscaler/types"

	"github.com/Fred78290/kubernetes-vmware-autoscaler/context"
	"gopkg.in/yaml.v2"
	apiv1 "k8s.io/api/core/v1"
)

// ShouldTestFeature check if test must be done
func ShouldTestFeature(name string) bool {
	if feature := os.Getenv(name); feature != "" {
		return feature != "NO"
	}

	return true
}

// MergeKubernetesLabel merge kubernetes map in one
func MergeKubernetesLabel(labels ...types.KubernetesLabel) types.KubernetesLabel {
	merged := types.KubernetesLabel{}

	for _, label := range labels {
		for k, v := range label {
			merged[k] = v
		}
	}

	return merged
}

// NodeFromJSON deserialize a string to apiv1.Node
func NodeFromJSON(s string) (*apiv1.Node, error) {
	data := &apiv1.Node{}

	err := json.Unmarshal([]byte(s), &data)

	return data, err
}

// ToYAML serialize interface to yaml
func ToYAML(v interface{}) string {
	if v == nil {
		return ""
	}

	b, _ := yaml.Marshal(v)

	return string(b)
}

// ToJSON serialize interface to json
func ToJSON(v interface{}) string {
	if v == nil {
		return ""
	}

	b, _ := json.Marshal(v)

	return string(b)
}

func DirExistAndReadable(name string) bool {
	if len(name) == 0 {
		return false
	}

	if entry, err := os.Stat(name); err != nil {
		return false
	} else if entry.IsDir() {

		if files, err := os.ReadDir(name); err == nil {
			for _, file := range files {
				if entry, err := file.Info(); err == nil {
					if !entry.IsDir() {
						fm := entry.Mode()
						sys := entry.Sys().(*syscall.Stat_t)

						if (fm&(1<<2) != 0) || ((fm&(1<<5)) != 0 && os.Getegid() == int(sys.Gid)) || ((fm&(1<<8)) != 0 && (os.Geteuid() == int(sys.Uid))) {
							continue
						}
					} else if DirExistAndReadable(name + "/" + entry.Name()) {
						continue
					}
				}

				return false
			}

			return true
		}
	}

	return false
}

func FileExistAndReadable(name string) bool {
	if len(name) == 0 {
		return false
	}

	if entry, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	} else {
		fm := entry.Mode()
		sys := entry.Sys().(*syscall.Stat_t)

		if (fm&(1<<2) != 0) || ((fm&(1<<5)) != 0 && os.Getegid() == int(sys.Gid)) || ((fm&(1<<8)) != 0 && (os.Geteuid() == int(sys.Uid))) {
			return true
		}
	}

	return false
}

// FileExists Check if FileExists
func FileExists(name string) bool {
	if len(name) == 0 {
		return false
	}

	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}

	return true
}

// MinInt min(a,b)
func MinInt(x, y int) int {
	if x < y {
		return x
	}
	return y
}

// MaxInt max(a,b)
func MaxInt(x, y int) int {
	if x > y {
		return x
	}
	return y
}

// MinInt64 min(a,b)
func MinInt64(x, y int64) int64 {
	if x < y {
		return x
	}
	return y
}

// MaxInt64 max(a,b)
func MaxInt64(x, y int64) int64 {
	if x > y {
		return x
	}
	return y
}

func NewRequestContext(requestTimeout time.Duration) *context.Context {
	return context.NewContext(time.Duration(requestTimeout.Seconds()))
}

// Values returns the values of the map m.
// The values will be in an indeterminate order.
func Values[M ~map[K]V, K comparable, V any](m M) []V {
	r := make([]V, 0, len(m))
	for _, v := range m {
		r = append(r, v)
	}
	return r
}
