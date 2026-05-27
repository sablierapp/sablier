package kubernetes

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/sablierapp/sablier/pkg/sablier"
)

// ClusterInspect reports the state of a CloudNativePG Cluster. The status is derived
// from the hibernation annotation and the cluster's ready/desired instance counts:
//   - hibernation "on"                        -> Stopped
//   - readyInstances >= spec.instances (>0)   -> Ready
//   - otherwise                               -> Starting
func (p *Provider) ClusterInspect(ctx context.Context, config ParsedName) (sablier.InstanceInfo, error) {
	if p.dynamic == nil {
		return sablier.InstanceInfo{}, fmt.Errorf("cnpgcluster support requires a dynamic client, none configured")
	}

	u, err := p.dynamic.Resource(cnpgClusterGVR).Namespace(config.Namespace).Get(ctx, config.Name, metav1.GetOptions{})
	if err != nil {
		return sablier.InstanceInfo{}, fmt.Errorf("error getting cnpg cluster: %w", err)
	}

	instances, _, _ := unstructured.NestedInt64(u.Object, "spec", "instances")
	if instances == 0 {
		instances = 1 // CloudNativePG defaults spec.instances to 1 when unset
	}
	readyInstances, _, _ := unstructured.NestedInt64(u.Object, "status", "readyInstances")
	image, _, _ := unstructured.NestedString(u.Object, "spec", "imageName")

	hibernation := u.GetAnnotations()[cnpgHibernationAnnotation]

	var status sablier.InstanceStatus
	switch {
	case hibernation == cnpgHibernationOn:
		status = sablier.InstanceStatusStopped
	case readyInstances >= instances:
		status = sablier.InstanceStatusReady
	default:
		status = sablier.InstanceStatusStarting
	}

	p.l.DebugContext(ctx, "cnpg cluster inspected",
		"cluster", config.Name, "namespace", config.Namespace,
		"instances", instances, "readyInstances", readyInstances, "hibernation", hibernation,
	)

	info := sablier.InstanceInfo{
		Name:            config.Original,
		CurrentReplicas: int32(readyInstances),
		DesiredReplicas: config.Replicas,
		Status:          status,
		Provider:        sablier.ProviderKubernetes,
	}

	labels := u.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	sablier.PopulateEnabledAndGroup(&info, labels)
	info.Kubernetes = &sablier.KubernetesWorkloadInfo{
		Namespace: config.Namespace,
		Kind:      KindCNPGCluster,
		Image:     image,
		Labels:    labels,
	}

	return info, nil
}
