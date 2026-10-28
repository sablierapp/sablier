package kubernetes

import (
	"context"
	"fmt"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type Workload interface {
	GetScale(ctx context.Context, workloadName string, options metav1.GetOptions) (*autoscalingv1.Scale, error)
	UpdateScale(ctx context.Context, workloadName string, scale *autoscalingv1.Scale, opts metav1.UpdateOptions) (*autoscalingv1.Scale, error)
}

func (p *Provider) scale(ctx context.Context, config ParsedName, replicas int32) error {
	// A merge patch on the scale subresource carries no resourceVersion, so it
	// cannot fail with a 409 Conflict when the workload is written concurrently
	// (e.g. the controller updating .status right after a previous scale).
	// Sablier sets an absolute replica count, so it has nothing to lock on.
	patch := []byte(fmt.Sprintf(`{"spec":{"replicas":%d}}`, replicas))

	// The typed clients decode the returned Scale into a Deployment or
	// StatefulSet, which leaves it empty; only the error is used.
	var err error
	switch config.Kind {
	case "deployment":
		_, err = p.Client.AppsV1().Deployments(config.Namespace).Patch(ctx, config.Name, types.MergePatchType, patch, metav1.PatchOptions{}, "scale")
	case "statefulset":
		_, err = p.Client.AppsV1().StatefulSets(config.Namespace).Patch(ctx, config.Name, types.MergePatchType, patch, metav1.PatchOptions{}, "scale")
	default:
		return fmt.Errorf("unsupported kind \"%s\" must be one of \"deployment\", \"statefulset\"", config.Kind)
	}

	return err
}
