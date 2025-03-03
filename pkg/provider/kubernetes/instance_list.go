package kubernetes

import (
	"context"
	"github.com/sablierapp/sablier/app/types"
	"github.com/sablierapp/sablier/pkg/provider"
)

func (p *KubernetesProvider) InstanceList(ctx context.Context, options provider.InstanceListOptions) ([]types.Instance, error) {
	deployments, err := p.DeploymentList(ctx, options)
	if err != nil {
		return nil, err
	}

	statefulSets, err := p.StatefulSetList(ctx, options)
	if err != nil {
		return nil, err
	}

	return append(deployments, statefulSets...), nil
}
