package kubernetes

import (
	"context"
	"slices"

	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
)

func (p *Provider) InstanceEvents(ctx context.Context, opts provider.InstanceEventsOptions) sablier.InstanceEventStream {
	wantStopped := len(opts.Types) == 0 || slices.Contains(opts.Types, provider.InstanceEventStopped)
	wantStarted := len(opts.Types) == 0 || slices.Contains(opts.Types, provider.InstanceEventStarted)
	wantCreated := len(opts.Types) == 0 || slices.Contains(opts.Types, provider.InstanceEventCreated)
	wantRemoved := len(opts.Types) == 0 || slices.Contains(opts.Types, provider.InstanceEventRemoved)
	eventsC := make(chan sablier.InstanceEvent)
	errC := make(chan error, 1)
	informer := p.watchDeployments(ctx, eventsC, wantStopped, wantStarted, wantCreated, wantRemoved)
	go informer.Run(ctx.Done())
	informer = p.watchStatefulSets(ctx, eventsC, wantStopped, wantStarted, wantCreated, wantRemoved)
	go informer.Run(ctx.Done())
	return sablier.InstanceEventStream{Events: eventsC, Err: errC}
}
