package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"

	"github.com/golang/glog"
	apiv1 "k8s.io/api/core/v1"
)

// NodeFromJSON deserialize a string to apiv1.Node
func nodeFromJSON(s string) (*apiv1.Node, error) {
	data := &apiv1.Node{}

	err := json.Unmarshal([]byte(s), &data)

	return data, err
}

func toJSON(v interface{}) string {
	if v == nil {
		return ""
	}

	b, _ := json.Marshal(v)

	return string(b)
}

func getNodeProviderID(serverIdentifier string, node *apiv1.Node) string {
	providerID := node.Spec.ProviderID

	if len(providerID) == 0 {
		nodegroupName := node.Labels[nodeLabelGroupName]

		if len(nodegroupName) != 0 {
			providerID = fmt.Sprintf("%s://%s/object?type=node&name=%s", serverIdentifier, nodegroupName, node.Name)
			glog.Infof("Warning misconfiguration: node providerID: %s is extracted from node label.", providerID)
		}
	}

	return providerID
}

func nodeGroupIDFromProviderID(serverIdentifier string, providerID string) (string, error) {
	var nodeIdentifier *url.URL
	var err error

	if nodeIdentifier, err = url.ParseRequestURI(providerID); err != nil {
		return "", err
	}

	if nodeIdentifier == nil {
		return "", fmt.Errorf(errCantDecodeNodeID, providerID)
	}

	if nodeIdentifier.Scheme != serverIdentifier {
		return "", fmt.Errorf(errWrongSchemeInProviderID, providerID, nodeIdentifier.Scheme)
	}

	if nodeIdentifier.Path != "object" && nodeIdentifier.Path != "/object" {
		return "", fmt.Errorf(errWrongPathInProviderID, providerID, nodeIdentifier.Path)
	}

	return nodeIdentifier.Hostname(), nil
}

func nodeNameFromProviderID(serverIdentifier string, providerID string) (string, error) {
	var nodeIdentifier *url.URL
	var err error

	if nodeIdentifier, err = url.ParseRequestURI(providerID); err != nil {
		return "", err
	}

	if nodeIdentifier == nil {
		return "", fmt.Errorf(errCantDecodeNodeID, providerID)
	}

	if nodeIdentifier.Scheme != serverIdentifier {
		return "", fmt.Errorf(errWrongSchemeInProviderID, providerID, nodeIdentifier.Scheme)
	}

	if nodeIdentifier.Path != "object" && nodeIdentifier.Path != "/object" {
		return "", fmt.Errorf(errWrongPathInProviderID, providerID, nodeIdentifier.Path)
	}

	return nodeIdentifier.Query().Get("name"), nil
}

func fileExists(name string) bool {
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

func minInt(x, y int) int {
	if x < y {
		return x
	}
	return y
}

func maxInt(x, y int) int {
	if x > y {
		return x
	}
	return y
}
