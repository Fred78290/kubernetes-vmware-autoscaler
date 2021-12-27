package v1alpha1

import (
	"context"
	"time"

	"github.com/Fred78290/kubernetes-vmware-autoscaler/api/types/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

type ManagedNodeInterface interface {
	List(ctx context.Context, opts metav1.ListOptions) (*v1alpha1.ManagedNodeList, error)
	Get(ctx context.Context, name string, opts metav1.GetOptions) (*v1alpha1.ManagedNode, error)
	Delete(ctx context.Context, node *v1alpha1.ManagedNode, opts metav1.DeleteOptions) error
	Create(ctx context.Context, node *v1alpha1.ManagedNode, opts metav1.CreateOptions) (*v1alpha1.ManagedNode, error)
	Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error)
	Update(ctx context.Context, node *v1alpha1.ManagedNode, opts metav1.UpdateOptions) (*v1alpha1.ManagedNode, error)
	UpdateStatus(ctx context.Context, node *v1alpha1.ManagedNode, opts metav1.UpdateOptions) (*v1alpha1.ManagedNode, error)
	// ...
}

type managedNodeClient struct {
	restClient     rest.Interface
	requestTimeout time.Duration
	ns             string
}

const ressourceName = "managednodes"

func NewManagedNodeInterface(client rest.Interface, requestTimeout time.Duration, namespace string) ManagedNodeInterface {
	return &managedNodeClient{
		restClient:     client,
		requestTimeout: requestTimeout,
		ns:             namespace,
	}
}

func (c *managedNodeClient) List(ctx context.Context, opts metav1.ListOptions) (*v1alpha1.ManagedNodeList, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}

	result := v1alpha1.ManagedNodeList{}
	err := c.restClient.
		Get().
		Namespace(c.ns).
		Resource(ressourceName).
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(&result)

	return &result, err
}

func (c *managedNodeClient) Get(ctx context.Context, name string, opts metav1.GetOptions) (*v1alpha1.ManagedNode, error) {
	result := v1alpha1.ManagedNode{}
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

func (c *managedNodeClient) Create(ctx context.Context, project *v1alpha1.ManagedNode, opts metav1.CreateOptions) (*v1alpha1.ManagedNode, error) {
	result := v1alpha1.ManagedNode{}
	err := c.restClient.
		Post().
		Namespace(c.ns).
		Resource(ressourceName).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(project).
		Do(ctx).
		Into(&result)

	return &result, err
}

func (c *managedNodeClient) Delete(ctx context.Context, project *v1alpha1.ManagedNode, opts metav1.DeleteOptions) error {
	return c.restClient.
		Delete().
		Namespace(c.ns).
		Resource(ressourceName).
		Name(project.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Do(ctx).Error()
}

func (c *managedNodeClient) Update(ctx context.Context, project *v1alpha1.ManagedNode, opts metav1.UpdateOptions) (*v1alpha1.ManagedNode, error) {
	result := v1alpha1.ManagedNode{}
	err := c.restClient.
		Put().
		Namespace(c.ns).
		Resource(ressourceName).
		Name(project.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(project).
		Do(ctx).
		Into(&result)

	return &result, err
}

func (c *managedNodeClient) UpdateStatus(ctx context.Context, project *v1alpha1.ManagedNode, opts metav1.UpdateOptions) (*v1alpha1.ManagedNode, error) {
	result := v1alpha1.ManagedNode{}

	err := c.restClient.
		Put().
		Namespace(c.ns).
		Resource(ressourceName).
		Name(project.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		SubResource("status").
		Body(project).
		Do(ctx).
		Into(&result)

	return &result, err
}

func (c *managedNodeClient) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.restClient.
		Get().
		Namespace(c.ns).
		Resource(ressourceName).
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(context.Background())
}
