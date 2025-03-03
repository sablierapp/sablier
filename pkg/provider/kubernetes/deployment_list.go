package kubernetes

import (
	"context"
	"github.com/sablierapp/sablier/app/discovery"
	"github.com/sablierapp/sablier/app/types"
	"github.com/sablierapp/sablier/pkg/provider"
	v1 "k8s.io/api/apps/v1"
	core_v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
)

func (p *KubernetesProvider) DeploymentList(ctx context.Context, options provider.InstanceListOptions) ([]types.Instance, error) {
	deployments, err := p.Client.AppsV1().Deployments(core_v1.NamespaceAll).List(ctx, metav1.ListOptions{
		LabelSelector: strings.Join(options.Labels, ","),
	})

	if err != nil {
		return nil, err
	}

	instances := make([]types.Instance, 0, len(deployments.Items))
	for _, d := range deployments.Items {
		instance := p.deploymentToInstance(&d)
		instances = append(instances, instance)
	}

	return instances, nil
}

func (p *KubernetesProvider) deploymentToInstance(d *v1.Deployment) types.Instance {
	var group string

	if _, ok := d.Labels[discovery.LabelEnable]; ok {
		if g, ok := d.Labels[discovery.LabelGroup]; ok {
			group = g
		} else {
			group = discovery.LabelGroupDefaultValue
		}
	}

	parsed := DeploymentName(d, ParseOptions{Delimiter: p.delimiter})

	return types.Instance{
		Name:  parsed.Original,
		Group: group,
	}
}
