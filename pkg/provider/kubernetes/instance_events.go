package kubernetes

import (
	"context"
	"slices"
	"sync"

	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
	"k8s.io/client-go/tools/cache"
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

	// Run returns once the informer has stopped and its handlers have
	// returned, so the channels can be closed after every Run returns, as the
	// Provider interface requires on context cancellation.
	var wg sync.WaitGroup
	run := func(informer cache.SharedIndexInformer) {
		wg.Go(func() { informer.Run(ctx.Done()) })
	}
	run(p.watchDeployments(ctx, eventsC, wantStopped, wantStarted, wantCreated, wantRemoved))
	run(p.watchStatefulSets(ctx, eventsC, wantStopped, wantStarted, wantCreated, wantRemoved))
	// Only watch CloudNativePG Clusters when the CRD is present, to avoid noisy
	// list/watch errors on clusters that don't run CloudNativePG.
	if p.clusterCRDInstalled(ctx) {
		run(p.watchClusters(ctx, eventsC, wantStopped, wantStarted, wantCreated, wantRemoved))
	}
	go func() {
		wg.Wait()
		close(eventsC)
		close(errC)
	}()
	return sablier.InstanceEventStream{Events: eventsC, Err: errC}
}

// emit sends ev on instance unless ctx is done, so an event handler never
// blocks once the stream's reader is gone and the informer can stop.
func emit(ctx context.Context, instance chan<- sablier.InstanceEvent, ev sablier.InstanceEvent) {
	select {
	case instance <- ev:
	case <-ctx.Done():
	}
}
