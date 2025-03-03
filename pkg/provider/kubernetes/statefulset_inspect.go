package kubernetes

import (
	"context"
	"github.com/sablierapp/sablier/app/instance"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (p *KubernetesProvider) StatefulSetInspect(ctx context.Context, config ParsedName) (instance.State, error) {
	ss, err := p.Client.AppsV1().StatefulSets(config.Namespace).Get(ctx, config.Name, metav1.GetOptions{})
	if err != nil {
		return instance.State{}, err
	}

	if *ss.Spec.Replicas != 0 && *ss.Spec.Replicas == ss.Status.ReadyReplicas {
		return instance.ReadyInstanceState(config.Original, ss.Status.ReadyReplicas), nil
	}

	return instance.NotReadyInstanceState(config.Original, ss.Status.ReadyReplicas, config.Replicas), nil
}
