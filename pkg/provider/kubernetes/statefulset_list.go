package kubernetes

import (
	"context"

	"github.com/sablierapp/sablier/pkg/sablier"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (p *Provider) StatefulSetList(ctx context.Context) ([]sablier.InstanceConfiguration, error) {
	labelSelector := metav1.LabelSelector{
		MatchLabels: map[string]string{
			sablier.LabelEnable: "true",
		},
	}
	statefulSets, err := p.Client.AppsV1().StatefulSets(corev1.NamespaceAll).List(ctx, metav1.ListOptions{
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

func (p *Provider) statefulSetToInstance(ss *v1.StatefulSet) sablier.InstanceConfiguration {
	config := sablierConfig(ss.Labels, ss.Annotations)
	enabled := config[sablier.LabelEnable]
	var groups []string
	if enabled == "true" {
		groups = sablier.ParseGroups(config[sablier.LabelGroup])
	}

	parsed := StatefulSetName(ss, ParseOptions{Delimiter: p.delimiter})

	return sablier.InstanceConfiguration{
		Name:      parsed.Original,
		Groups:    groups,
		Enabled:   enabled,
		Delegated: delegatedScaling(config),
	}
}

func (p *Provider) StatefulSetGroups(ctx context.Context) (map[string][]string, error) {
	labelSelector := metav1.LabelSelector{
		MatchLabels: map[string]string{
			sablier.LabelEnable: "true",
		},
	}
	statefulSets, err := p.Client.AppsV1().StatefulSets(corev1.NamespaceAll).List(ctx, metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(&labelSelector),
	})
	if err != nil {
		return nil, err
	}

	groups := make(map[string][]string)
	for _, ss := range statefulSets.Items {
		parsed := StatefulSetName(&ss, ParseOptions{Delimiter: p.delimiter})
		config := sablierConfig(ss.Labels, ss.Annotations)
		for _, groupName := range sablier.ParseGroups(config[sablier.LabelGroup]) {
			groups[groupName] = append(groups[groupName], parsed.Original)
		}
	}

	return groups, nil
}
