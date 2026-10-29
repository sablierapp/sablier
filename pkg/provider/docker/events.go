package docker

import (
	"context"
	"slices"
	"strings"

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
	wantUpdated := len(opts.Types) == 0 || slices.Contains(opts.Types, provider.InstanceEventUpdated)
	wantRemoved := len(opts.Types) == 0 || slices.Contains(opts.Types, provider.InstanceEventRemoved)

	dial := func(ctx context.Context) client.EventsResult {
		filters := client.Filters{}
		filters.Add("type", string(events.ContainerEventType))
		if wantStopped {
			filters.Add("event", string(events.ActionDie))
		}
		if wantStarted {
			filters.Add("event", string(events.ActionStart))
		}
		if wantCreated {
			filters.Add("event", string(events.ActionCreate))
		}
		if wantRemoved {
			filters.Add("event", string(events.ActionDestroy))
		}
		// A rename is reported as a removal followed by an update.
		if wantUpdated || wantRemoved {
			filters.Add("event", string(events.ActionRename))
		}
		return p.Client.Events(ctx, client.EventsListOptions{Filters: filters})
	}
	build := func(ctx context.Context, msg events.Message) ([]sablier.InstanceEvent, bool) {
		name := strings.TrimPrefix(msg.Actor.Attributes["name"], "/")
		if name == "" {
			return nil, false
		}
		switch msg.Action {
		case events.ActionDie:
			if !wantStopped {
				return nil, false
			}
			info, err := p.InstanceInspect(ctx, name)
			if err != nil {
				p.l.WarnContext(ctx, "inspect after die event failed, using bare info", "container", name, "error", err)
				return []sablier.InstanceEvent{{Type: provider.InstanceEventStopped, Info: sablier.InstanceInfo{Name: name, Status: sablier.InstanceStatusStopped, Provider: sablier.ProviderDocker}}}, true
			}
			return []sablier.InstanceEvent{{Type: provider.InstanceEventStopped, Info: info}}, true
		case events.ActionStart:
			if !wantStarted {
				return nil, false
			}
			info, err := p.InstanceInspect(ctx, name)
			if err != nil {
				p.l.WarnContext(ctx, "inspect after start event failed, using bare info", "container", name, "error", err)
				return []sablier.InstanceEvent{{Type: provider.InstanceEventStarted, Info: sablier.InstanceInfo{Name: name, Status: sablier.InstanceStatusStarting, Provider: sablier.ProviderDocker}}}, true
			}
			return []sablier.InstanceEvent{{Type: provider.InstanceEventStarted, Info: info}}, true
		case events.ActionCreate:
			if !wantCreated {
				return nil, false
			}
			info, err := p.InstanceInspect(ctx, name)
			if err != nil {
				p.l.WarnContext(ctx, "inspect after create event failed, using bare info", "container", name, "error", err)
				return []sablier.InstanceEvent{{Type: provider.InstanceEventCreated, Info: sablier.InstanceInfo{Name: name, Provider: sablier.ProviderDocker}}}, true
			}
			return []sablier.InstanceEvent{{Type: provider.InstanceEventCreated, Info: info}}, true
		case events.ActionRename:
			// Compose recreates a container as "<id>_<name>", removes the old one,
			// then renames the replacement to "<name>". Report the old name as
			// removed, then the new one as updated so it joins its groups.
			oldName := strings.TrimPrefix(msg.Actor.Attributes["oldName"], "/")
			var out []sablier.InstanceEvent
			if wantRemoved && oldName != "" {
				out = append(out, sablier.InstanceEvent{Type: provider.InstanceEventRemoved, Info: sablier.InstanceInfo{Name: oldName, Provider: sablier.ProviderDocker}})
			}
			if wantUpdated {
				info, err := p.InstanceInspect(ctx, name)
				if err != nil {
					p.l.WarnContext(ctx, "inspect after rename event failed, using bare info", "container", name, "error", err)
					info = sablier.InstanceInfo{Name: name, Provider: sablier.ProviderDocker}
				}
				out = append(out, sablier.InstanceEvent{Type: provider.InstanceEventUpdated, Info: info})
			}
			return out, len(out) > 0
		case events.ActionDestroy:
			if !wantRemoved {
				return nil, false
			}
			// Container is already gone; only the name is available.
			return []sablier.InstanceEvent{{Type: provider.InstanceEventRemoved, Info: sablier.InstanceInfo{Name: name, Provider: sablier.ProviderDocker}}}, true
		default:
			p.l.WarnContext(ctx, "unhandled event", "action", msg.Action, "container", name)
			return nil, false
		}
	}
	return mobyevents.Stream(ctx, p.l, dial, build)
}
