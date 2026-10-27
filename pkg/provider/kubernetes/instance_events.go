package kubernetes

import (
	"context"
	"slices"

	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
)

// informerResync disables the periodic resync of the instance event informers.
// Their update handlers drop updates whose ResourceVersion did not change, so a
// resync only produces no-op notifications: one per watched object and period.
// client-go queues them without bound behind a handler that is blocked sending
// an event, which happens whenever the stream reader is busy.
const informerResync = 0

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
	// Only watch CloudNativePG Clusters when the CRD is present, to avoid noisy
	// list/watch errors on clusters that don't run CloudNativePG.
	if p.clusterCRDInstalled(ctx) {
		informer = p.watchClusters(ctx, eventsC, wantStopped, wantStarted, wantCreated, wantRemoved)
		go informer.Run(ctx.Done())
	}
	return sablier.InstanceEventStream{Events: eventsC, Err: errC}
}
