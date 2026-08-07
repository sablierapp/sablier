package dockerswarm

import (
	"context"
	"slices"

	"github.com/moby/moby/api/types/events"
	"github.com/moby/moby/client"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/provider/internal/mobyevents"
	"github.com/sablierapp/sablier/pkg/sablier"
)

func (p *Provider) InstanceEvents(ctx context.Context, opts provider.InstanceEventsOptions) sablier.InstanceEventStream {
	wantStopped := len(opts.Types) == 0 || slices.Contains(opts.Types, provider.InstanceEventStopped)
	wantStarted := len(opts.Types) == 0 || slices.Contains(opts.Types, provider.InstanceEventStarted)
	wantCreated := len(opts.Types) == 0 || slices.Contains(opts.Types, provider.InstanceEventCreated)
	wantRemoved := len(opts.Types) == 0 || slices.Contains(opts.Types, provider.InstanceEventRemoved)
	dial := func(ctx context.Context) client.EventsResult {
		filters := client.Filters{}
		filters.Add("scope", "swarm")
		filters.Add("type", "service")
		return p.Client.Events(ctx, client.EventsListOptions{Filters: filters})
	}
	build := func(ctx context.Context, msg events.Message) (sablier.InstanceEvent, bool) {
		name := msg.Actor.Attributes["name"]
		if name == "" {
			return sablier.InstanceEvent{}, false
		}
		switch msg.Action {
		case events.ActionCreate:
			if !wantCreated {
				return sablier.InstanceEvent{}, false
			}
			info, err := p.InstanceInspect(ctx, name)
			if err != nil {
				p.l.WarnContext(ctx, "inspect after create event failed, using bare info", "service", name, "error", err)
				return sablier.InstanceEvent{Type: provider.InstanceEventCreated, Info: sablier.InstanceInfo{Name: name, Provider: sablier.ProviderSwarm}}, true
			}
			return sablier.InstanceEvent{Type: provider.InstanceEventCreated, Info: info}, true
		case events.ActionRemove:
			// Service is already gone; only the name is available.
			if wantRemoved {
				return sablier.InstanceEvent{Type: provider.InstanceEventRemoved, Info: sablier.InstanceInfo{Name: name, Provider: sablier.ProviderSwarm}}, true
			}
			// Removal implies the instance is stopped — emit a stopped event for subscribers that care.
			if wantStopped {
				return sablier.InstanceEvent{Type: provider.InstanceEventStopped, Info: sablier.InstanceInfo{Name: name, Status: sablier.InstanceStatusStopped, Provider: sablier.ProviderSwarm}}, true
			}
			return sablier.InstanceEvent{}, false
		default:
			// "update" action — check replicas attributes for scale events.
			replicasNew := msg.Actor.Attributes["replicas.new"]
			replicasOld := msg.Actor.Attributes["replicas.old"]
			if replicasNew == "0" && wantStopped {
				info, err := p.InstanceInspect(ctx, name)
				if err != nil {
					p.l.WarnContext(ctx, "inspect after scale-to-0 event failed, using bare info", "service", name, "error", err)
					return sablier.InstanceEvent{Type: provider.InstanceEventStopped, Info: sablier.InstanceInfo{Name: name, Status: sablier.InstanceStatusStopped, Provider: sablier.ProviderSwarm}}, true
				}
				return sablier.InstanceEvent{Type: provider.InstanceEventStopped, Info: info}, true
			}
			if replicasOld == "0" && replicasNew != "" && replicasNew != "0" && wantStarted {
				info, err := p.InstanceInspect(ctx, name)
				if err != nil {
					p.l.WarnContext(ctx, "inspect after scale-from-0 event failed, using bare info", "service", name, "error", err)
					return sablier.InstanceEvent{Type: provider.InstanceEventStarted, Info: sablier.InstanceInfo{Name: name, Status: sablier.InstanceStatusStarting, Provider: sablier.ProviderSwarm}}, true
				}
				return sablier.InstanceEvent{Type: provider.InstanceEventStarted, Info: info}, true
			}
			return sablier.InstanceEvent{}, false
		}
	}
	return mobyevents.Stream(ctx, p.l, dial, build)
}
