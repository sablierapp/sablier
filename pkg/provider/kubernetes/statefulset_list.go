package kubernetes

import (
	"context"

	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (p *Provider) StatefulSetList(ctx context.Context, opts provider.InstanceListOptions) ([]sablier.InstanceConfiguration, error) {
	statefulSets, err := p.listStatefulSets(ctx)
	if err != nil {
		return nil, err
	}

	instances := make([]sablier.InstanceConfiguration, 0, len(statefulSets))
	for _, ss := range statefulSets {
		if !opts.All && !statefulSetIsRunning(&ss) {
			continue
		}
		instance := p.statefulSetToInstance(&ss)
		instances = append(instances, instance)
	}

	return instances, nil
}

// A nil replicas value defaults to 1 replica.
// https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/stateful-set-v1/#StatefulSetSpec
func statefulSetIsRunning(ss *v1.StatefulSet) bool {
	return ss.Spec.Replicas == nil || *ss.Spec.Replicas != 0
}

func (p *Provider) statefulSetToInstance(ss *v1.StatefulSet) sablier.InstanceConfiguration {
	config := sablierConfig(ss.Labels, ss.Annotations)
	enabled := config[sablier.LabelEnable]
	var groups []string
	if enabled == "true" {
		groups = sablier.ParseGroups(config[sablier.LabelGroup])
	}

	parsed := StatefulSetName(ss, ParseOptions{Delimiter: p.delimiter})

	return sablier.InstanceConfiguration{
		Name:    parsed.Original,
		Groups:  groups,
		Enabled: enabled,
	}
}

func (p *Provider) StatefulSetGroups(ctx context.Context) (map[string][]string, error) {
	statefulSets, err := p.listStatefulSets(ctx)
	if err != nil {
		return nil, err
	}

	groups := make(map[string][]string)
	for _, ss := range statefulSets {
		parsed := StatefulSetName(&ss, ParseOptions{Delimiter: p.delimiter})
		config := sablierConfig(ss.Labels, ss.Annotations)
		for _, groupName := range sablier.ParseGroups(config[sablier.LabelGroup]) {
			groups[groupName] = append(groups[groupName], parsed.Original)
		}
	}

	return groups, nil
}

func (p *Provider) listStatefulSets(ctx context.Context) ([]v1.StatefulSet, error) {
	return listEnabled(func(opts metav1.ListOptions) ([]v1.StatefulSet, error) {
		statefulSets, err := p.Client.AppsV1().StatefulSets(corev1.NamespaceAll).List(ctx, opts)
		if err != nil {
			return nil, err
		}
		return statefulSets.Items, nil
	})
}
