package kubernetes

import (
	"context"

	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (p *Provider) DeploymentList(ctx context.Context, opts provider.InstanceListOptions) ([]sablier.InstanceConfiguration, error) {
	deployments, err := p.listDeployments(ctx)
	if err != nil {
		return nil, err
	}

	instances := make([]sablier.InstanceConfiguration, 0, len(deployments))
	for _, d := range deployments {
		if !opts.All && !deploymentIsRunning(&d) {
			continue
		}
		instance := p.deploymentToInstance(&d)
		instances = append(instances, instance)
	}

	return instances, nil
}

// A nil replicas value defaults to 1 replica.
// https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/deployment-v1/#DeploymentSpec
func deploymentIsRunning(d *v1.Deployment) bool {
	return d.Spec.Replicas == nil || *d.Spec.Replicas != 0
}

func (p *Provider) deploymentToInstance(d *v1.Deployment) sablier.InstanceConfiguration {
	config := sablierConfig(d.Labels, d.Annotations)
	enabled := config[sablier.LabelEnable]
	var groups []string
	if enabled == "true" {
		groups = sablier.ParseGroups(config[sablier.LabelGroup])
	}

	parsed := DeploymentName(d, ParseOptions{Delimiter: p.delimiter})

	return sablier.InstanceConfiguration{
		Name:    parsed.Original,
		Groups:  groups,
		Enabled: enabled,
	}
}

func (p *Provider) DeploymentGroups(ctx context.Context) (map[string][]string, error) {
	deployments, err := p.listDeployments(ctx)
	if err != nil {
		return nil, err
	}

	groups := make(map[string][]string)
	for _, deployment := range deployments {
		parsed := DeploymentName(&deployment, ParseOptions{Delimiter: p.delimiter})
		config := sablierConfig(deployment.Labels, deployment.Annotations)
		for _, groupName := range sablier.ParseGroups(config[sablier.LabelGroup]) {
			groups[groupName] = append(groups[groupName], parsed.Original)
		}
	}

	return groups, nil
}

func (p *Provider) listDeployments(ctx context.Context) ([]v1.Deployment, error) {
	return listEnabled(func(opts metav1.ListOptions) ([]v1.Deployment, error) {
		deployments, err := p.Client.AppsV1().Deployments(corev1.NamespaceAll).List(ctx, opts)
		if err != nil {
			return nil, err
		}
		return deployments.Items, nil
	})
}
