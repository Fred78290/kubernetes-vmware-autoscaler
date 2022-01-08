package utils

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/Fred78290/kubernetes-vmware-autoscaler/constantes"
	"github.com/Fred78290/kubernetes-vmware-autoscaler/context"
	glog "github.com/sirupsen/logrus"
	apiv1 "k8s.io/api/core/v1"
)

// NodeFromJSON deserialize a string to apiv1.Node
func NodeFromJSON(s string) (*apiv1.Node, error) {
	data := &apiv1.Node{}

	err := json.Unmarshal([]byte(s), &data)

	return data, err
}

// ToJSON serialize interface to json
func ToJSON(v interface{}) string {
	if v == nil {
		return ""
	}

	b, _ := json.Marshal(v)

	return string(b)
}

// GetNodeProviderID func
func GetNodeProviderID(serverIdentifier string, node *apiv1.Node) string {
	providerID := node.Spec.ProviderID

	if len(providerID) == 0 {
		nodegroupName := node.Labels[constantes.NodeLabelGroupName]

		if len(nodegroupName) != 0 {
			providerID = fmt.Sprintf("%s://%s/object?type=node&name=%s", serverIdentifier, nodegroupName, node.Name)
			glog.Infof("Warning misconfiguration: node providerID: %s is extracted from node label.", providerID)
		}
	}

	return providerID
}

// NodeGroupIDFromProviderID returns group node name from provider
func NodeGroupIDFromProviderID(serverIdentifier string, providerID string) (string, error) {
	var nodeIdentifier *url.URL
	var err error

	if nodeIdentifier, err = url.ParseRequestURI(providerID); err != nil {
		return "", err
	}

	if nodeIdentifier == nil {
		return "", fmt.Errorf(constantes.ErrCantDecodeNodeID, providerID)
	}

	if nodeIdentifier.Scheme != serverIdentifier {
		return "", fmt.Errorf(constantes.ErrWrongSchemeInProviderID, providerID, serverIdentifier, nodeIdentifier.Scheme)
	}

	if nodeIdentifier.Path != "object" && nodeIdentifier.Path != "/object" {
		return "", fmt.Errorf(constantes.ErrWrongPathInProviderID, providerID, nodeIdentifier.Path)
	}

	return nodeIdentifier.Hostname(), nil
}

// NodeNameFromProviderID return node name from provider ID
func NodeNameFromProviderID(serverIdentifier string, providerID string) (string, error) {
	var nodeIdentifier *url.URL
	var err error

	if nodeIdentifier, err = url.ParseRequestURI(providerID); err != nil {
		return "", err
	}

	if nodeIdentifier == nil {
		return "", fmt.Errorf(constantes.ErrCantDecodeNodeID, providerID)
	}

	if nodeIdentifier.Scheme != serverIdentifier {
		return "", fmt.Errorf(constantes.ErrWrongSchemeInProviderID, providerID, serverIdentifier, nodeIdentifier.Scheme)
	}

	if nodeIdentifier.Path != "object" && nodeIdentifier.Path != "/object" {
		return "", fmt.Errorf(constantes.ErrWrongPathInProviderID, providerID, nodeIdentifier.Path)
	}

	return nodeIdentifier.Query().Get("name"), nil
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
