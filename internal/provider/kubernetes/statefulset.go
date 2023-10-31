package kubernetes

import (
	"context"
	"fmt"

	"github.com/acouvreur/sablier/internal/provider"
	v1 "k8s.io/api/apps/v1"
	core_v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (provider *Client) StatefulSetStatus(ctx context.Context, config ParsedName) (bool, error) {
	ss, err := provider.Client.AppsV1().
		StatefulSets(config.Namespace).
		Get(ctx, config.Name, metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	ready := *ss.Spec.Replicas == ss.Status.ReadyReplicas

	return ready, nil
}

func (client *Client) discoverStatefulSets(ctx context.Context, opts provider.DiscoveryOptions) ([]provider.Discovered, error) {
	statefulSets, err := client.Client.AppsV1().StatefulSets(core_v1.NamespaceAll).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=true", opts.EnableLabel),
	})
	if err != nil {
		return nil, err
	}

	discovered := make([]provider.Discovered, len(statefulSets.Items))
	for index, statefulSet := range statefulSets.Items {
		discovered[index] = client.toDiscoveredStatefulSet(statefulSet, opts)
	}
	return discovered, nil
}

func (client *Client) toDiscoveredStatefulSet(statefulSet v1.StatefulSet, opts provider.DiscoveryOptions) provider.Discovered {
	name := StatefulSetName(statefulSet, client.ParseOptions)
	var group string

	// The container defined a label with its named group
	if foundGroup, ok := statefulSet.Labels[opts.GroupLabel]; ok {
		group = foundGroup
	} else if opts.DefaultGroupStartegy == provider.DefaultGroupStartegyUseInstanceName {
		// The container did not define a label and uses the instance name as group
		group = name.Original
	} else if opts.DefaultGroupStartegy == provider.DefaultGroupStrategyUseValue {
		// The container did not define a label and uses the "default" group
		group = provider.DefaultGroupValue
	}

	return provider.Discovered{
		Name:  name.Original,
		Group: group,
	}
}
