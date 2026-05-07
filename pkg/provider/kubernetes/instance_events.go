package kubernetes

import (
	"context"

	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
)

func (p *Provider) InstanceEvents(ctx context.Context, _ provider.InstanceEventsOptions) sablier.InstanceEventStream {
	eventsC := make(chan sablier.InstanceInfo)
	errC := make(chan error, 1)
	informer := p.watchDeployments(eventsC)
	go informer.Run(ctx.Done())
	informer = p.watchStatefulSets(eventsC)
	go informer.Run(ctx.Done())
	return sablier.InstanceEventStream{Events: eventsC, Err: errC}
}
