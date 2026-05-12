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
	if *ss.Spec.Replicas != 0 && *ss.Spec.Replicas == ss.Status.ReadyReplicas {
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

	sablier.PopulateEnabledAndGroup(&info, ss.Labels)

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
