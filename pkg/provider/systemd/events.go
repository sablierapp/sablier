package systemd

import (
	"context"
	"log/slog"
	"slices"

	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
)

const eventBufferSize = 16

func (p *Provider) InstanceEvents(ctx context.Context, opts provider.InstanceEventsOptions) sablier.InstanceEventStream {
	wantStopped := len(opts.Types) == 0 || slices.Contains(opts.Types, provider.InstanceEventStopped)
	wantStarted := len(opts.Types) == 0 || slices.Contains(opts.Types, provider.InstanceEventStarted)
	wantRemoved := len(opts.Types) == 0 || slices.Contains(opts.Types, provider.InstanceEventRemoved)

	eventsC := make(chan sablier.InstanceEvent)
	errC := make(chan error, 1)

	// The subscription polls ListUnits every pollInterval and reports changed
	// units; removed units arrive with a nil status. When unit patterns are
	// configured, unrelated units are filtered out before the diff so the
	// baseline and every poll only touch matching units.
	var filter func(string) bool
	if len(p.unitPatterns) > 0 {
		filter = func(name string) bool { return !p.matchUnitPattern(name) }
	}
	changes, errs := p.Con.SubscribeUnitsCustomContext(ctx, p.pollInterval, eventBufferSize, unitStatusChanged, filter)

	go func() {
		defer close(eventsC)
		defer close(errC)
		for {
			select {
			case changed, ok := <-changes:
				if !ok {
					return
				}
				for name, status := range changed {
					event, ok := p.buildEvent(ctx, name, status, wantStopped, wantStarted, wantRemoved)
					if !ok {
						continue
					}
					select {
					case eventsC <- event:
					case <-ctx.Done():
						return
					}
				}
			case err, ok := <-errs:
				if !ok {
					return
				}
				p.l.WarnContext(ctx, "systemd event stream error", slog.Any("error", err))
			case <-ctx.Done():
				return
			}
		}
	}()

	return sablier.InstanceEventStream{Events: eventsC, Err: errC}
}

// unitStatusChanged reports only transitions into a final state.
func unitStatusChanged(u1, u2 *dbus.UnitStatus) bool {
	return u1 != nil && u2 != nil && u1.ActiveState != u2.ActiveState && isFinalState(u2.ActiveState)
}

func isFinalState(state string) bool {
	switch state {
	case "activating", "deactivating", "reloading", "refreshing":
		return false
	}
	return true
}

func (p *Provider) buildEvent(ctx context.Context, name string, status *dbus.UnitStatus, wantStopped, wantStarted, wantRemoved bool) (sablier.InstanceEvent, bool) {
	var kind provider.InstanceEventType
	switch {
	case status == nil:
		if !wantRemoved {
			return sablier.InstanceEvent{}, false
		}
		return sablier.InstanceEvent{
			Type: provider.InstanceEventRemoved,
			Info: sablier.InstanceInfo{Name: name, Provider: sablier.ProviderSystemd},
		}, true
	case status.LoadState == "not-found":
		return sablier.InstanceEvent{}, false
	case status.ActiveState == "active":
		if !wantStarted {
			return sablier.InstanceEvent{}, false
		}
		kind = provider.InstanceEventStarted
	case status.ActiveState == "inactive", status.ActiveState == "failed", status.ActiveState == "maintenance":
		if !wantStopped {
			return sablier.InstanceEvent{}, false
		}
		kind = provider.InstanceEventStopped
	default:
		return sablier.InstanceEvent{}, false
	}

	info, err := p.InstanceInspect(ctx, name)
	if err != nil {
		p.l.WarnContext(ctx, "inspect after unit change failed, using bare info", slog.String("unit", name), slog.Any("error", err))
		info = sablier.InstanceInfo{Name: name, Provider: sablier.ProviderSystemd}
		switch kind {
		case provider.InstanceEventStarted:
			info.Status = sablier.InstanceStatusStarting
		case provider.InstanceEventStopped:
			info.Status = sablier.InstanceStatusStopped
		}
	}
	return sablier.InstanceEvent{Type: kind, Info: info}, true
}
