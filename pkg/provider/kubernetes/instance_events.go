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

// InstanceEvents reports the lifecycle events of the deployments, statefulsets
// and CloudNativePG clusters that opted in to Sablier. Other workloads are
// ignored (see managed).
func (p *Provider) InstanceEvents(ctx context.Context, opts provider.InstanceEventsOptions) sablier.InstanceEventStream {
	wantStopped := len(opts.Types) == 0 || slices.Contains(opts.Types, provider.InstanceEventStopped)
	wantStarted := len(opts.Types) == 0 || slices.Contains(opts.Types, provider.InstanceEventStarted)
	wantCreated := len(opts.Types) == 0 || slices.Contains(opts.Types, provider.InstanceEventCreated)
	wantRemoved := len(opts.Types) == 0 || slices.Contains(opts.Types, provider.InstanceEventRemoved)
	eventsC := make(chan sablier.InstanceEvent)
	errC := make(chan error, 1)

	// The informer handlers send to an internal channel. One goroutine
	// forwards its events until the context is done, then keeps reading and
	// dropping them, so a handler never blocks once the reader is gone and
	// every informer can stop. Run returns after its handlers have returned:
	// once every Run has returned, internal is closed, and so is the stream,
	// as the Provider interface requires on context cancellation.
	internal := make(chan sablier.InstanceEvent)
	var wg sync.WaitGroup
	run := func(informer cache.SharedIndexInformer) {
		wg.Go(func() { informer.Run(ctx.Done()) })
	}
	run(p.watchDeployments(ctx, internal, wantStopped, wantStarted, wantCreated, wantRemoved))
	run(p.watchStatefulSets(ctx, internal, wantStopped, wantStarted, wantCreated, wantRemoved))
	// Only watch CloudNativePG Clusters when the CRD is present, to avoid noisy
	// list/watch errors on clusters that don't run CloudNativePG.
	if p.clusterCRDInstalled(ctx) {
		run(p.watchClusters(ctx, internal, wantStopped, wantStarted, wantCreated, wantRemoved))
	}
	go func() {
		wg.Wait()
		close(internal)
	}()
	go func() {
		defer close(errC)
		defer close(eventsC)
		for ev := range internal {
			select {
			case eventsC <- ev:
			case <-ctx.Done():
			}
		}
	}()
	return sablier.InstanceEventStream{Events: eventsC, Err: errC}
}
