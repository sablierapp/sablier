package dockerswarm

import (
	"context"
	"log/slog"
	"slices"

	"github.com/moby/moby/api/types/events"
	"github.com/moby/moby/client"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/provider/internal/mobyevents"
	"github.com/sablierapp/sablier/pkg/sablier"
)

// InstanceEvents reports the lifecycle events of the services that have
// sablier.enable=true. Swarm service events carry the service name but not its
// labels, so each relevant event is followed by an inspect.
func (p *Provider) InstanceEvents(ctx context.Context, opts provider.InstanceEventsOptions) sablier.InstanceEventStream {
	want := func(t provider.InstanceEventType) bool {
		return len(opts.Types) == 0 || slices.Contains(opts.Types, t)
	}
	// A remove event is reported as removed, or as stopped when removed is not wanted.
	wantRemoval := want(provider.InstanceEventRemoved) || want(provider.InstanceEventStopped)

	// managed holds the names of the services last seen with sablier.enable=true.
	// A removed service can no longer be inspected, so this is the only way to
	// tell whether its removal must be reported. dial and build run one after
	// the other on the stream goroutine, so the map needs no lock.
	managed := make(map[string]bool)

	dial := func(ctx context.Context) client.EventsResult {
		filters := client.Filters{}
		filters.Add("scope", "swarm")
		filters.Add("type", "service")
		result := p.Client.Events(ctx, client.EventsListOptions{Filters: filters})
		// Seed after subscribing, so a service created in between is caught by
		// its create event. Names are only added: a managed service removed
		// between the two calls must still have its removal reported.
		p.seedManaged(ctx, managed)
		return result
	}

	build := func(ctx context.Context, msg events.Message) (sablier.InstanceEvent, bool) {
		name := msg.Actor.Attributes["name"]
		if name == "" {
			return sablier.InstanceEvent{}, false
		}

		var bare sablier.InstanceInfo
		var t provider.InstanceEventType
		switch msg.Action {
		case events.ActionCreate:
			t = provider.InstanceEventCreated
			bare = sablier.InstanceInfo{Name: name, Provider: sablier.ProviderSwarm}
		case events.ActionRemove:
			return removalEvent(managed, name, want)
		default:
			// "update" action — check replicas attributes for scale events.
			replicasNew := msg.Actor.Attributes["replicas.new"]
			replicasOld := msg.Actor.Attributes["replicas.old"]
			switch {
			case replicasNew == "0":
				t = provider.InstanceEventStopped
				bare = sablier.InstanceInfo{Name: name, Status: sablier.InstanceStatusStopped, Provider: sablier.ProviderSwarm}
			case replicasOld == "0" && replicasNew != "":
				t = provider.InstanceEventStarted
				bare = sablier.InstanceInfo{Name: name, Status: sablier.InstanceStatusStarting, Provider: sablier.ProviderSwarm}
			default:
				return sablier.InstanceEvent{}, false
			}
		}

		// Inspect even when t is not wanted if removals are: it keeps managed
		// up to date for services created or labeled after the stream started.
		if !want(t) && !wantRemoval {
			return sablier.InstanceEvent{}, false
		}
		info, err := p.InstanceInspect(ctx, name)
		switch {
		case err != nil && managed[name]:
			// Most likely removed right after the event. It was managed, so
			// report it with what the event tells.
			p.l.WarnContext(ctx, "inspect after event failed, using bare info", "service", name, "action", msg.Action, "error", err)
			info = bare
		case err != nil:
			// Without the labels there is no telling whether the service is
			// managed. Reporting it could leak an unmanaged service, which is
			// what this stream must not do.
			p.l.DebugContext(ctx, "inspect after event failed, skipping service not known to be managed", "service", name, "action", msg.Action, "error", err)
			return sablier.InstanceEvent{}, false
		case info.IsEnabled():
			managed[name] = true
		default:
			delete(managed, name)
			return sablier.InstanceEvent{}, false
		}

		if !want(t) {
			return sablier.InstanceEvent{}, false
		}
		return sablier.InstanceEvent{Type: t, Info: info}, true
	}
	return mobyevents.Stream(ctx, p.l, dial, build)
}

// seedManaged adds the services that have sablier.enable=true to managed.
func (p *Provider) seedManaged(ctx context.Context, managed map[string]bool) {
	instances, err := p.InstanceList(ctx, provider.InstanceListOptions{All: true})
	if err != nil {
		p.l.WarnContext(ctx, "cannot list managed services, removals of services not seen since will not be reported", slog.Any("error", err))
		return
	}
	for _, instance := range instances {
		if instance.IsEnabled() {
			managed[instance.Name] = true
		}
	}
}

// removalEvent reports the removal of a service, which is already gone and
// only known by its name. It is reported only if the service was managed.
func removalEvent(managed map[string]bool, name string, want func(provider.InstanceEventType) bool) (sablier.InstanceEvent, bool) {
	if !managed[name] {
		return sablier.InstanceEvent{}, false
	}
	delete(managed, name)
	if want(provider.InstanceEventRemoved) {
		return sablier.InstanceEvent{Type: provider.InstanceEventRemoved, Info: sablier.InstanceInfo{Name: name, Provider: sablier.ProviderSwarm}}, true
	}
	// Removal implies the instance is stopped — emit a stopped event for subscribers that care.
	if want(provider.InstanceEventStopped) {
		return sablier.InstanceEvent{Type: provider.InstanceEventStopped, Info: sablier.InstanceInfo{Name: name, Status: sablier.InstanceStatusStopped, Provider: sablier.ProviderSwarm}}, true
	}
	return sablier.InstanceEvent{}, false
}
