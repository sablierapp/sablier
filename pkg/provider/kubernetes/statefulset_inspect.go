package kubernetes

import (
	"context"

	"github.com/sablierapp/sablier/pkg/sablier"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (p *Provider) StatefulSetInspect(ctx context.Context, config ParsedName) (sablier.InstanceInfo, error) {
	ss, err := p.Client.AppsV1().StatefulSets(config.Namespace).Get(ctx, config.Name, metav1.GetOptions{})
	if err != nil {
		return sablier.InstanceInfo{}, err
	}

	var info sablier.InstanceInfo
	ready := *ss.Spec.Replicas != 0 && *ss.Spec.Replicas == ss.Status.ReadyReplicas
	if p.readyOnFirstReplica {
		// A workload scaled to zero must still be reported as stopped, even if
		// terminating pods transiently keep readyReplicas above zero.
		ready = *ss.Spec.Replicas != 0 && ss.Status.ReadyReplicas > 0
	}
	if ready {
		info = sablier.InstanceInfo{
			Name:            config.Original,
			CurrentReplicas: ss.Status.ReadyReplicas,
			DesiredReplicas: config.Replicas,
			Status:          sablier.InstanceStatusReady,
		}
	} else if *ss.Spec.Replicas == 0 {
		info = sablier.InstanceInfo{
			Name:            config.Original,
			CurrentReplicas: ss.Status.ReadyReplicas,
			DesiredReplicas: config.Replicas,
			Status:          sablier.InstanceStatusStopped,
		}
	} else {
		info = sablier.InstanceInfo{
			Name:            config.Original,
			CurrentReplicas: ss.Status.ReadyReplicas,
			DesiredReplicas: config.Replicas,
			Status:          sablier.InstanceStatusStarting,
		}
	}

	sablier.PopulateEnabledAndGroup(&info, sablierConfig(ss.Labels, ss.Annotations))

	var image string
	if len(ss.Spec.Template.Spec.Containers) > 0 {
		image = ss.Spec.Template.Spec.Containers[0].Image
	}
	info.Provider = sablier.ProviderKubernetes
	labels := ss.Labels
	if labels == nil {
		labels = map[string]string{}
	}
	info.Kubernetes = &sablier.KubernetesWorkloadInfo{
		Namespace: config.Namespace,
		Kind:      "statefulset",
		Image:     image,
		Labels:    labels,
	}

	return info, nil
}
