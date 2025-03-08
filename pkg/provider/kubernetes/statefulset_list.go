package kubernetes

import (
	"context"
	"github.com/sablierapp/sablier/app/discovery"
	"github.com/sablierapp/sablier/pkg/sablier"
	v1 "k8s.io/api/apps/v1"
	core_v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (p *KubernetesProvider) StatefulSetList(ctx context.Context) ([]sablier.InstanceConfiguration, error) {
	labelSelector := metav1.LabelSelector{
		MatchLabels: map[string]string{
			discovery.LabelEnable: "true",
		},
	}
	statefulSets, err := p.Client.AppsV1().StatefulSets(core_v1.NamespaceAll).List(ctx, metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(&labelSelector),
	})
	if err != nil {
		return nil, err
	}

	instances := make([]sablier.InstanceConfiguration, 0, len(statefulSets.Items))
	for _, ss := range statefulSets.Items {
		instance := p.statefulSetToInstance(&ss)
		instances = append(instances, instance)
	}

	return instances, nil
}

func (p *KubernetesProvider) statefulSetToInstance(ss *v1.StatefulSet) sablier.InstanceConfiguration {
	var group string

	if _, ok := ss.Labels[discovery.LabelEnable]; ok {
		if g, ok := ss.Labels[discovery.LabelGroup]; ok {
			group = g
		} else {
			group = discovery.LabelGroupDefaultValue
		}
	}

	parsed := StatefulSetName(ss, ParseOptions{Delimiter: p.delimiter})

	return sablier.InstanceConfiguration{
		Name:  parsed.Original,
		Group: group,
	}
}

func (p *KubernetesProvider) StatefulSetGroups(ctx context.Context) (map[string][]string, error) {
	labelSelector := metav1.LabelSelector{
		MatchLabels: map[string]string{
			discovery.LabelEnable: "true",
		},
	}
	statefulSets, err := p.Client.AppsV1().StatefulSets(core_v1.NamespaceAll).List(ctx, metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(&labelSelector),
	})
	if err != nil {
		return nil, err
	}

	groups := make(map[string][]string)
	for _, ss := range statefulSets.Items {
		groupName := ss.Labels[discovery.LabelGroup]
		if len(groupName) == 0 {
			groupName = discovery.LabelGroupDefaultValue
		}

		group := groups[groupName]
		parsed := StatefulSetName(&ss, ParseOptions{Delimiter: p.delimiter})
		group = append(group, parsed.Original)
		groups[groupName] = group
	}

	return groups, nil
}
