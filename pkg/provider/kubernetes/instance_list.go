package kubernetes

import (
	"context"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
	"maps"
)

func (p *Provider) InstanceList(ctx context.Context, options provider.InstanceListOptions) ([]sablier.InstanceConfiguration, error) {
	deployments, err := p.DeploymentList(ctx)
	if err != nil {
		return nil, err
	}

	statefulSets, err := p.StatefulSetList(ctx)
	if err != nil {
		return nil, err
	}

	clusters, err := p.ClusterList(ctx)
	if err != nil {
		return nil, err
	}

	instances := append(deployments, statefulSets...)
	instances = append(instances, clusters...)
	return instances, nil
}

func (p *Provider) InstanceGroups(ctx context.Context) (map[string][]string, error) {
	deployments, err := p.DeploymentGroups(ctx)
	if err != nil {
		return nil, err
	}

	statefulSets, err := p.StatefulSetGroups(ctx)
	if err != nil {
		return nil, err
	}

	clusters, err := p.ClusterGroups(ctx)
	if err != nil {
		return nil, err
	}

	groups := make(map[string][]string)
	maps.Copy(groups, deployments)

	for group, instances := range statefulSets {
		groups[group] = append(groups[group], instances...)
	}

	for group, instances := range clusters {
		groups[group] = append(groups[group], instances...)
	}

	return groups, nil
}
