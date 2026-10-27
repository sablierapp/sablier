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

	cfg := sablierConfig(d.Labels, d.Annotations)
	desired := desiredReplicas(cfg, config)

	var info sablier.InstanceInfo
	ready := *d.Spec.Replicas != 0 && *d.Spec.Replicas == d.Status.ReadyReplicas
	if p.readyOnFirstReplica {
		// A workload scaled to zero must still be reported as stopped, even if
		// terminating pods transiently keep readyReplicas above zero.
		ready = *d.Spec.Replicas != 0 && d.Status.ReadyReplicas > 0
	}
	if ready {
		info = sablier.InstanceInfo{
			Name:            config.Original,
			CurrentReplicas: d.Status.ReadyReplicas,
			DesiredReplicas: desired,
			Status:          sablier.InstanceStatusReady,
		}
	} else if *d.Spec.Replicas == 0 {
		info = sablier.InstanceInfo{
			Name:            config.Original,
			CurrentReplicas: d.Status.ReadyReplicas,
			DesiredReplicas: desired,
			Status:          sablier.InstanceStatusStopped,
		}
	} else {
		info = sablier.InstanceInfo{
			Name:            config.Original,
			CurrentReplicas: d.Status.ReadyReplicas,
			DesiredReplicas: desired,
			Status:          sablier.InstanceStatusStarting,
		}
	}

	sablier.PopulateEnabledAndGroup(&info, cfg)

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
		Name:      config.Name,
		Image:     image,
		Labels:    labels,
	}

	return info, nil
}
