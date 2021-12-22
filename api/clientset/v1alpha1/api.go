package v1alpha1

import (
	"github.com/Fred78290/kubernetes-vmware-autoscaler/api/types/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

type ScalerNodeV1Alpha1Interface interface {
	ScalerNodes(namespace string) ScalerNodeInterface
}

type ScalerNodeV1Alpha1Client struct {
	restClient rest.Interface
}

func NewForConfig(c *rest.Config) (*ScalerNodeV1Alpha1Client, error) {
	config := *c
	config.ContentConfig.GroupVersion = &schema.GroupVersion{Group: v1alpha1.GroupName, Version: v1alpha1.GroupVersion}
	config.APIPath = "/apis"
	config.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
	config.UserAgent = rest.DefaultKubernetesUserAgent()

	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}

	return &ScalerNodeV1Alpha1Client{restClient: client}, nil
}

func (c *ScalerNodeV1Alpha1Client) Projects(namespace string) ScalerNodeInterface {
	return &scalerNodeClient{
		restClient: c.restClient,
		ns:         namespace,
	}
}
