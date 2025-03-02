package kubernetes

import (
	"context"
	"fmt"
	"github.com/sablierapp/sablier/app/instance"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (p *KubernetesProvider) DeploymentInspect(ctx context.Context, config ParsedName) (instance.State, error) {
	d, err := p.Client.AppsV1().Deployments(config.Namespace).Get(ctx, config.Name, metav1.GetOptions{})
	if err != nil {
		return instance.State{}, fmt.Errorf("error getting deployment: %w", err)
	}
	
	// TODO: Should add option to set ready as soon as one replica is ready
	if *d.Spec.Replicas != 0 && *d.Spec.Replicas == d.Status.ReadyReplicas {
		return instance.ReadyInstanceState(config.Original, config.Replicas), nil
	}

	return instance.NotReadyInstanceState(config.Original, d.Status.ReadyReplicas, config.Replicas), nil
}
