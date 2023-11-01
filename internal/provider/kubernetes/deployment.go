package kubernetes

import (
	"context"
	"fmt"

	"github.com/acouvreur/sablier/internal/provider"
	v1 "k8s.io/api/apps/v1"
	core_v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (provider *Client) DeploymentStatus(ctx context.Context, config ParsedName) (bool, error) {
	d, err := provider.Client.AppsV1().
		Deployments(config.Namespace).
		Get(ctx, config.Name, metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	ready := *d.Spec.Replicas == d.Status.ReadyReplicas

	return ready, nil
}

func (client *Client) discoverDeployments(ctx context.Context, opts provider.DiscoveryOptions) ([]provider.Discovered, error) {
	deployments, err := client.Client.AppsV1().Deployments(core_v1.NamespaceAll).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=true", opts.EnableLabel),
	})
	if err != nil {
		return nil, err
	}

	discovered := make([]provider.Discovered, len(deployments.Items))
	for index, deployment := range deployments.Items {
		discovered[index] = client.toDiscoveredDeployment(deployment, opts)
	}
	return discovered, nil
}

func (client *Client) toDiscoveredDeployment(deployment v1.Deployment, opts provider.DiscoveryOptions) provider.Discovered {
	name := DeploymentName(deployment, client.ParseOptions)
	var group string

	// The container defined a label with its named group
	if foundGroup, ok := deployment.Labels[opts.GroupLabel]; ok {
		group = foundGroup
	} else if opts.DefaultGroupStrategy == provider.DefaultGroupStrategyUseInstanceName {
		// The container did not define a label and uses the instance name as group
		group = name.Original
	} else if opts.DefaultGroupStrategy == provider.DefaultGroupStrategyUseValue {
		// The container did not define a label and uses the "default" group
		group = provider.DefaultGroupValue
	}

	return provider.Discovered{
		Name:  name.Original,
		Group: group,
	}
}
