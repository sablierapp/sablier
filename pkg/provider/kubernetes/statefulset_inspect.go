package kubernetes

import (
	"context"
	"github.com/sablierapp/sablier/pkg/sablier"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (p *KubernetesProvider) StatefulSetInspect(ctx context.Context, config ParsedName) (sablier.InstanceInfo, error) {
	ss, err := p.Client.AppsV1().StatefulSets(config.Namespace).Get(ctx, config.Name, metav1.GetOptions{})
	if err != nil {
		return sablier.InstanceInfo{}, err
	}

	if *ss.Spec.Replicas != 0 && *ss.Spec.Replicas == ss.Status.ReadyReplicas {
		return sablier.ReadyInstanceState(config.Original, ss.Status.ReadyReplicas), nil
	}

	return sablier.NotReadyInstanceState(config.Original, ss.Status.ReadyReplicas, config.Replicas), nil
}
