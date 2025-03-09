package kubernetes

import (
	"context"
	"github.com/sablierapp/sablier/pkg/sablier"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (p *KubernetesProvider) DeploymentList(ctx context.Context) ([]sablier.InstanceConfiguration, error) {
	labelSelector := metav1.LabelSelector{
		MatchLabels: map[string]string{
			"sablier.enable": "true",
		},
	}
	deployments, err := p.Client.AppsV1().Deployments(corev1.NamespaceAll).List(ctx, metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(&labelSelector),
	})
	if err != nil {
		return nil, err
	}

	instances := make([]sablier.InstanceConfiguration, 0, len(deployments.Items))
	for _, d := range deployments.Items {
		instance := p.deploymentToInstance(&d)
		instances = append(instances, instance)
	}

	return instances, nil
}

func (p *KubernetesProvider) deploymentToInstance(d *v1.Deployment) sablier.InstanceConfiguration {
	var group string

	if _, ok := d.Labels["sablier.enable"]; ok {
		if g, ok := d.Labels["sablier.group"]; ok {
			group = g
		} else {
			group = "default"
		}
	}

	parsed := DeploymentName(d, ParseOptions{Delimiter: p.delimiter})

	return sablier.InstanceConfiguration{
		Name:  parsed.Original,
		Group: group,
	}
}

func (p *KubernetesProvider) DeploymentGroups(ctx context.Context) (map[string][]string, error) {
	labelSelector := metav1.LabelSelector{
		MatchLabels: map[string]string{
			"sablier.enable": "true",
		},
	}
	deployments, err := p.Client.AppsV1().Deployments(corev1.NamespaceAll).List(ctx, metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(&labelSelector),
	})

	if err != nil {
		return nil, err
	}

	groups := make(map[string][]string)
	for _, deployment := range deployments.Items {
		groupName := deployment.Labels["sablier.group"]
		if len(groupName) == 0 {
			groupName = "default"
		}

		group := groups[groupName]
		parsed := DeploymentName(&deployment, ParseOptions{Delimiter: p.delimiter})
		group = append(group, parsed.Original)
		groups[groupName] = group
	}

	return groups, nil
}
