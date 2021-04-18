package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Fred78290/kubernetes-vmware-autoscaler/constantes"
	"github.com/Fred78290/kubernetes-vmware-autoscaler/types"
	"github.com/linki/instrumented_http"
	glog "github.com/sirupsen/logrus"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type patchStringValue struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value bool   `json:"value"`
}

// SingletonClientGenerator provides clients
type SingletonClientGenerator struct {
	KubeConfig     string
	APIServerURL   string
	RequestTimeout time.Duration
	kubeClient     kubernetes.Interface
	kubeOnce       sync.Once
}

// Context wrapper
type clientContext struct {
	context.Context
}

func (ctx *clientContext) Cancel() {

}

func newContext(timeout time.Duration) *clientContext {
	return &clientContext{
		context.TODO(),
	}
}

// getRestConfig returns the rest clients config to get automatically
// data if you run inside a cluster or by passing flags.
func getRestConfig(kubeConfig, apiServerURL string) (*rest.Config, error) {
	if kubeConfig == "" {
		if _, err := os.Stat(clientcmd.RecommendedHomeFile); err == nil {
			kubeConfig = clientcmd.RecommendedHomeFile
		}
	}
	glog.Debugf("apiServerURL: %s", apiServerURL)
	glog.Debugf("kubeConfig: %s", kubeConfig)

	// evaluate whether to use kubeConfig-file or serviceaccount-token
	var (
		config *rest.Config
		err    error
	)
	if kubeConfig == "" {
		glog.Infof("Using inCluster-config based on serviceaccount-token")
		config, err = rest.InClusterConfig()
	} else {
		glog.Infof("Using kubeConfig")
		config, err = clientcmd.BuildConfigFromFlags(apiServerURL, kubeConfig)
	}
	if err != nil {
		return nil, err
	}

	return config, nil
}

// NewKubeClient returns a new Kubernetes client object. It takes a Config and
// uses APIServerURL and KubeConfig attributes to connect to the cluster. If
// KubeConfig isn't provided it defaults to using the recommended default.
func newKubeClient(kubeConfig, apiServerURL string, requestTimeout time.Duration) (*kubernetes.Clientset, error) {
	glog.Infof("Instantiating new Kubernetes client")
	config, err := getRestConfig(kubeConfig, apiServerURL)
	if err != nil {
		return nil, err
	}

	config.Timeout = requestTimeout * time.Second

	config.WrapTransport = func(rt http.RoundTripper) http.RoundTripper {
		return instrumented_http.NewTransport(rt, &instrumented_http.Callbacks{
			PathProcessor: func(path string) string {
				parts := strings.Split(path, "/")
				return parts[len(parts)-1]
			},
		})
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	glog.Infof("Created Kubernetes client %s", config.Host)

	return client, nil
}

// KubeClient generates a kube client if it was not created before
func (p *SingletonClientGenerator) KubeClient() (kubernetes.Interface, error) {
	var err error
	p.kubeOnce.Do(func() {
		p.kubeClient, err = newKubeClient(p.KubeConfig, p.APIServerURL, p.RequestTimeout)
	})
	return p.kubeClient, err
}

func (p *SingletonClientGenerator) WaitNodeToBeReady(nodeName string, timeToWaitInSeconds int) error {
	var nodeInfo *apiv1.Node
	kubeclient, err := p.KubeClient()

	if err != nil {
		return err
	}

	ctx := newContext(p.RequestTimeout)
	defer ctx.Cancel()

	timeout := time.Duration(timeToWaitInSeconds) * time.Second

	for t := time.Now(); time.Since(t) < timeout; time.Sleep(2 * time.Second) {
		nodeInfo, err = kubeclient.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})

		if err != nil {
			return err
		}

		for _, status := range nodeInfo.Status.Conditions {
			if status.Type == "Ready" {
				if b, e := strconv.ParseBool(string(status.Status)); e == nil {
					if b {
						glog.Infof("The kubernetes node %s is Ready", nodeName)
						return nil
					}
				}
			}
		}

		glog.Infof("The kubernetes node:%s is not ready", nodeName)
	}

	return fmt.Errorf(constantes.ErrNodeIsNotReady, nodeName)
}

// NodeList return node list from cluster
func (p *SingletonClientGenerator) NodeList() (*apiv1.NodeList, error) {

	kubeclient, err := p.KubeClient()

	if err != nil {
		return nil, err
	}

	ctx := newContext(p.RequestTimeout)
	defer ctx.Cancel()

	return kubeclient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
}

func (p *SingletonClientGenerator) cordonOrUncordonNode(nodeName string, flag bool) error {
	kubeclient, err := p.KubeClient()

	if err != nil {
		return err
	}

	ctx := newContext(p.RequestTimeout)
	defer ctx.Cancel()

	if _, err = kubeclient.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{}); err != nil {
		return err
	}

	payload := []patchStringValue{{
		Op:    "replace",
		Path:  "/spec/unschedulable",
		Value: flag,
	}}

	payloadBytes, _ := json.Marshal(payload)

	_, err = kubeclient.CoreV1().Nodes().Patch(ctx, nodeName, apitypes.JSONPatchType, payloadBytes, metav1.PatchOptions{})

	return err
}

func (p *SingletonClientGenerator) UncordonNode(nodeName string) error {
	return p.cordonOrUncordonNode(nodeName, false)
}

func (p *SingletonClientGenerator) CordonNode(nodeName string) error {
	return p.cordonOrUncordonNode(nodeName, true)
}

func (p *SingletonClientGenerator) DrainNode(nodeName string) error {
	return nil
}

func (p *SingletonClientGenerator) DeleteNode(nodeName string) error {
	kubeclient, err := p.KubeClient()

	if err != nil {
		return err
	}

	ctx := newContext(p.RequestTimeout)
	defer ctx.Cancel()

	return kubeclient.CoreV1().Nodes().Delete(ctx, nodeName, metav1.DeleteOptions{})
}

// AnnoteNode set annotation on node
func (p *SingletonClientGenerator) AnnoteNode(nodeName string, annotations map[string]string) error {
	var nodeInfo *apiv1.Node

	kubeclient, err := p.KubeClient()

	if err != nil {
		return err
	}

	ctx := newContext(p.RequestTimeout)
	defer ctx.Cancel()

	if nodeInfo, err = kubeclient.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{}); err != nil {
		return err
	}

	if len(nodeInfo.Annotations) == 0 {
		nodeInfo.Annotations = annotations
	} else {
		for k, v := range annotations {
			nodeInfo.Annotations[k] = v
		}
	}

	_, err = kubeclient.CoreV1().Nodes().Update(ctx, nodeInfo, metav1.UpdateOptions{})

	return err
}

// AnnoteNode set annotation on node
func (p *SingletonClientGenerator) LabelNode(nodeName string, labels map[string]string) error {
	var nodeInfo *apiv1.Node

	kubeclient, err := p.KubeClient()

	if err != nil {
		return err
	}

	ctx := newContext(p.RequestTimeout)
	defer ctx.Cancel()

	if nodeInfo, err = kubeclient.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{}); err != nil {
		return err
	}

	if len(nodeInfo.Labels) == 0 {
		nodeInfo.Labels = labels
	} else {
		for k, v := range labels {
			nodeInfo.Labels[k] = v
		}
	}

	_, err = kubeclient.CoreV1().Nodes().Update(ctx, nodeInfo, metav1.UpdateOptions{})

	return err
}

func NewClientGenerator(cfg *types.Config) *SingletonClientGenerator {
	return &SingletonClientGenerator{
		KubeConfig:     cfg.KubeConfig,
		APIServerURL:   cfg.APIServerURL,
		RequestTimeout: cfg.RequestTimeout,
	}
}
