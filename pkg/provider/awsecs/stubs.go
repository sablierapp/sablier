package awsecs

import (
	"context"

	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
)

// Temporary stubs so the package compiles while the remaining methods are
// implemented. Deleted at the end of the events task.

func (p *Provider) InstanceList(_ context.Context, _ provider.InstanceListOptions) ([]sablier.InstanceConfiguration, error) {
	return nil, nil
}

func (p *Provider) InstanceGroups(_ context.Context) (map[string][]string, error) {
	return nil, nil
}

func (p *Provider) InstanceStart(_ context.Context, _ string) error {
	return nil
}

func (p *Provider) InstanceStop(_ context.Context, _ string) error {
	return nil
}

func (p *Provider) InstanceDependencies(_ context.Context, _ string) ([]sablier.InstanceDependency, error) {
	return nil, nil
}

func (p *Provider) InstanceEvents(_ context.Context, _ provider.InstanceEventsOptions) sablier.InstanceEventStream {
	return sablier.InstanceEventStream{}
}
