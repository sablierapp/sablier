package kubernetes

import (
	"context"
	"fmt"

	"github.com/sablierapp/sablier/pkg/sablier"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (p *Provider) DeploymentInspect(ctx context.Context, config ParsedName) (sablier.InstanceInfo, error) {
	d, err := p.Client.AppsV1().Deployments(config.Namespace).Get(ctx, config.Name, metav1.GetOptions{})
	if err != nil {
		return sablier.InstanceInfo{}, fmt.Errorf("error getting deployment: %w", err)
	}

	p.l.DebugContext(ctx, "deployment inspected", "deployment", config.Name, "namespace", config.Namespace, "replicas", d.Status.Replicas, "readyReplicas", d.Status.ReadyReplicas, "availableReplicas", d.Status.AvailableReplicas)

	var info sablier.InstanceInfo
	ready := *d.Spec.Replicas != 0 && *d.Spec.Replicas == d.Status.ReadyReplicas
	if p.readyOnFirstReplica {
		ready = d.Status.ReadyReplicas > 0
	}
	if ready {
		info = sablier.InstanceInfo{
			Name:            config.Original,
			CurrentReplicas: config.Replicas,
			DesiredReplicas: config.Replicas,
			Status:          sablier.InstanceStatusReady,
		}
	} else if *d.Spec.Replicas == 0 {
		info = sablier.InstanceInfo{
			Name:            config.Original,
			CurrentReplicas: d.Status.ReadyReplicas,
			DesiredReplicas: config.Replicas,
			Status:          sablier.InstanceStatusStopped,
		}
	} else {
		info = sablier.InstanceInfo{
			Name:            config.Original,
			CurrentReplicas: d.Status.ReadyReplicas,
			DesiredReplicas: config.Replicas,
			Status:          sablier.InstanceStatusStarting,
		}
	}

	sablier.PopulateEnabledAndGroup(&info, sablierConfig(d.Labels, d.Annotations))

	var image string
	if len(d.Spec.Template.Spec.Containers) > 0 {
		image = d.Spec.Template.Spec.Containers[0].Image
	}
	info.Provider = sablier.ProviderKubernetes
	labels := d.Labels
	if labels == nil {
		labels = map[string]string{}
	}
	info.Kubernetes = &sablier.KubernetesWorkloadInfo{
		Namespace: config.Namespace,
		Kind:      "deployment",
		Image:     image,
		Labels:    labels,
	}

	return info, nil
}
