package v1alpha1

import (
	"time"

	"github.com/Fred78290/kubernetes-vmware-autoscaler/api/types/v1alpha1"
	"github.com/Fred78290/kubernetes-vmware-autoscaler/context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

type ScalerNodeInterface interface {
	List(opts metav1.ListOptions) (*v1alpha1.ScaledNodeList, error)
	Get(name string, options metav1.GetOptions) (*v1alpha1.ScaledNode, error)
	Create(*v1alpha1.ScaledNode) (*v1alpha1.ScaledNode, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	// ...
}

type scalerNodeClient struct {
	restClient     rest.Interface
	requestTimeout time.Duration
	ns             string
}

const ressourceName = "scalednode"

func NewScalerNodeInterface(client rest.Interface, requestTimeout time.Duration, namespace string) ScalerNodeInterface {
	return &scalerNodeClient{
		restClient:     client,
		requestTimeout: requestTimeout,
		ns:             namespace,
	}
}

func (c *scalerNodeClient) newRequestContext() *context.Context {
	return context.NewContext(time.Duration(c.requestTimeout.Seconds()))
}

func (c *scalerNodeClient) List(opts metav1.ListOptions) (*v1alpha1.ScaledNodeList, error) {
	ctx := c.newRequestContext()

	defer ctx.Cancel()

	result := v1alpha1.ScaledNodeList{}
	err := c.restClient.
		Get().
		Namespace(c.ns).
		Resource(ressourceName).
		VersionedParams(&opts, scheme.ParameterCodec).
		Do(ctx).
		Into(&result)

	return &result, err
}

func (c *scalerNodeClient) Get(name string, opts metav1.GetOptions) (*v1alpha1.ScaledNode, error) {
	ctx := c.newRequestContext()

	defer ctx.Cancel()

	result := v1alpha1.ScaledNode{}
	err := c.restClient.
		Get().
		Namespace(c.ns).
		Resource(ressourceName).
		Name(name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Do(ctx).
		Into(&result)

	return &result, err
}

func (c *scalerNodeClient) Create(project *v1alpha1.ScaledNode) (*v1alpha1.ScaledNode, error) {
	ctx := c.newRequestContext()

	defer ctx.Cancel()

	result := v1alpha1.ScaledNode{}
	err := c.restClient.
		Post().
		Namespace(c.ns).
		Resource(ressourceName).
		Body(project).
		Do(ctx).
		Into(&result)

	return &result, err
}

func (c *scalerNodeClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.restClient.
		Get().
		Namespace(c.ns).
		Resource(ressourceName).
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch(context.Background())
}
