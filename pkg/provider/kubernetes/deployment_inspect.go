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

	// TODO: Should add option to set ready as soon as one replica is ready
	if *d.Spec.Replicas != 0 && *d.Spec.Replicas == d.Status.ReadyReplicas {
		return sablier.ReadyInstanceState(config.Original, config.Replicas), nil
	}

	return sablier.NotReadyInstanceState(config.Original, d.Status.ReadyReplicas, config.Replicas), nil
}
