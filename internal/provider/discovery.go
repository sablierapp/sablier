package provider

import (
	"context"
	log "log/slog"
	"sync"
)

const (
	EnableLabel       string = "sablier.enable"
	GroupLabel        string = "sablier.group"
	DefaultGroupValue string = "default"
)

type DefaultGroupStrategy string

const (
	DefaultGroupStrategyUseInstanceName DefaultGroupStrategy = "instanceName"
	DefaultGroupStrategyUseValue        DefaultGroupStrategy = "value"
)

type DiscoveryOptions struct {
	EnableLabel          string
	GroupLabel           string
	DefaultGroupStrategy DefaultGroupStrategy
	StopOnDiscover       bool
}

type Label struct {
	Key   string
	Value string
}

type Discovered struct {
	Name  string
	Group string
}

type Discovery struct {
	provider Client
	opts     DiscoveryOptions
	groups   map[string][]string
	lock     *sync.Mutex
}

func NewDiscovery(provider Client, opts DiscoveryOptions) *Discovery {
	return &Discovery{
		provider: provider,
		opts:     opts,
		groups:   map[string][]string{},
		lock:     &sync.Mutex{},
	}
}

// StartDiscovery retrieves from the provider all the available instances
func (d *Discovery) StartDiscovery(ctx context.Context) {
	// Initial scan
	d.scan(ctx)

	// Start watching and rescan on event
	ch, errs := d.provider.Events(ctx)

	select {
	case <-ctx.Done():
		return
	case msg := <-ch:
		log.InfoContext(ctx, msg.Name, msg.Action)
	case err := <-errs:
		log.ErrorContext(ctx, err.Error())
	}

}

func (d *Discovery) scan(ctx context.Context) {
	discovereds, err := d.provider.Discover(ctx, d.opts)
	if err != nil {
		return
	}

	groups := make(map[string][]string, len(discovereds))
	for _, discovered := range discovereds {
		group, ok := groups[discovered.Group]
		if !ok {
			group = make([]string, 0)
		}
		group = append(group, discovered.Name)
		groups[discovered.Group] = group

		if d.opts.StopOnDiscover {
			err := d.provider.Stop(ctx, discovered.Name)
			if err != nil {
				log.Warn("could not stop instance", "instance", discovered.Name)
			}
		}
	}

	d.lock.Lock()
	defer d.lock.Unlock()
	d.groups = groups
	log.InfoContext(ctx, "scan complete", "groups", groups)
}

func (d *Discovery) Group(name string) ([]string, bool) {
	d.lock.Lock()
	defer d.lock.Unlock()
	group, ok := d.groups[name]
	return group, ok
}

func (d *Discovery) Groups() map[string][]string {
	d.lock.Lock()
	defer d.lock.Unlock()
	return d.groups
}
