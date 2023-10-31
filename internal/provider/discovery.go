package provider

import (
	"context"
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

func NewDiscovery(opts DiscoveryOptions) Discovery {
	return Discovery{
		opts: opts,
	}
}

// StartDiscovery retrieves from the provider all the available instances
func (d *Discovery) StartDiscovery() {
	// Initial scan
	d.refresh()

	// Start watching and rescan on event
	d.provider.Events(context.Background())
}

func (d *Discovery) refresh() {

}

func (d *Discovery) Group(name string) ([]string, bool) {
	d.lock.Lock()
	defer d.lock.Unlock()
	group, ok := d.groups[name]
	return group, ok
}
