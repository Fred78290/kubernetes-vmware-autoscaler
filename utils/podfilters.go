package utils

import (
	"context"
	"fmt"
	"strings"

	"github.com/Fred78290/kubernetes-vmware-autoscaler/types"
	apiv1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	kindDaemonSet   = "DaemonSet"
	kindStatefulSet = "StatefulSet"
)

// MirrorPodFilter returns true if the supplied pod is not a mirror pod, i.e. a
// pod created by a manifest on the node rather than the API server.
func MirrorPodFilter(p apiv1.Pod) (bool, error) {
	_, mirrorPod := p.GetAnnotations()[apiv1.MirrorPodAnnotationKey]
	return !mirrorPod, nil
}

// LocalStoragePodFilter returns true if the supplied pod does not have local
// storage, i.e. does not use any 'empty dir' volumes.
func LocalStoragePodFilter(p apiv1.Pod) (bool, error) {
	for _, v := range p.Spec.Volumes {
		if v.EmptyDir != nil {
			return false, nil
		}
	}
	return true, nil
}

// UnreplicatedPodFilter returns true if the pod is replicated, i.e. is managed
// by a controller (deployment, daemonset, statefulset, etc) of some sort.
func UnreplicatedPodFilter(p apiv1.Pod) (bool, error) {
	// We're fine with 'evicting' unreplicated pods that aren't actually running.
	if p.Status.Phase == apiv1.PodSucceeded || p.Status.Phase == apiv1.PodFailed {
		return true, nil
	}
	if metav1.GetControllerOf(&p) == nil {
		return false, nil
	}
	return true, nil
}

// NewDaemonSetPodFilter returns a FilterFunc that returns true if the supplied
// pod is not managed by an extant DaemonSet.
func NewDaemonSetPodFilter(ctx context.Context, client kubernetes.Interface) types.PodFilterFunc {
	return func(p apiv1.Pod) (bool, error) {
		c := metav1.GetControllerOf(&p)
		if c == nil || c.Kind != kindDaemonSet {
			return true, nil
		}

		// Pods pass the filter if they were created by a DaemonSet that no
		// longer exists.
		if _, err := client.AppsV1().DaemonSets(p.GetNamespace()).Get(ctx, c.Name, metav1.GetOptions{}); err != nil {
			if apierrors.IsNotFound(err) {
				return true, nil
			}
			return false, fmt.Errorf("cannot get DaemonSet %s/%s, reason: %v", p.GetNamespace(), c.Name, err)
		}
		return false, nil
	}
}

// NewStatefulSetPodFilter returns a FilterFunc that returns true if the supplied
// pod is not managed by an extant StatefulSet.
func NewStatefulSetPodFilter(ctx context.Context, client kubernetes.Interface) types.PodFilterFunc {
	return func(p apiv1.Pod) (bool, error) {
		c := metav1.GetControllerOf(&p)
		if c == nil || c.Kind != kindStatefulSet {
			return true, nil
		}

		// Pods pass the filter if they were created by a StatefulSet that no
		// longer exists.
		if _, err := client.AppsV1().StatefulSets(p.GetNamespace()).Get(ctx, c.Name, metav1.GetOptions{}); err != nil {
			if apierrors.IsNotFound(err) {
				return true, nil
			}
			return false, fmt.Errorf("cannot get StatefulSet %s/%s, reason: %v", p.GetNamespace(), c.Name, err)
		}
		return false, nil
	}
}

// UnprotectedPodFilter returns a FilterFunc that returns true if the
// supplied pod does not have any of the user-specified annotations for
// protection from eviction
func UnprotectedPodFilter(annotations ...string) types.PodFilterFunc {
	return func(p apiv1.Pod) (bool, error) {
		var filter bool
		for _, annot := range annotations {
			// Try to split the annotation into key-value pairs
			kv := strings.SplitN(annot, "=", 2)
			if len(kv) < 2 {
				// If the annotation is a single string, then simply check for
				// the existence of the annotation key
				_, filter = p.GetAnnotations()[kv[0]]
			} else {
				// If the annotation is a key-value pair, then check if the
				// value for the pod annotation matches that of the
				// user-specified value
				v, ok := p.GetAnnotations()[kv[0]]
				filter = ok && v == kv[1]
			}
			if filter {
				return false, nil
			}
		}
		return true, nil
	}
}

// NewPodFilters returns a FilterFunc that returns true if all of the supplied
// FilterFuncs return true.
func NewPodFilters(filters ...types.PodFilterFunc) types.PodFilterFunc {
	return func(p apiv1.Pod) (bool, error) {
		for _, fn := range filters {
			passes, err := fn(p)
			if err != nil {
				return false, fmt.Errorf("cannot apply filters, reason: %v", err)
			}
			if !passes {
				return false, nil
			}
		}
		return true, nil
	}
}
