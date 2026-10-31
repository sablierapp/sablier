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

		// managed records whether each unit was last seen with Enable=true.
		// A removed unit has no unit file left to read, so this is how its
		// removal is reported for managed units only. The first poll reports
		// every unit, which seeds it. Only this goroutine touches it.
		managed := make(map[string]bool)
		for {
			select {
			case changed, ok := <-changes:
				if !ok {
					return
				}
				for name, status := range changed {
					event, ok := p.buildEvent(ctx, managed, name, status, wantStopped, wantStarted, wantRemoved)
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

// buildEvent turns a unit change into an event. Only units that opted in with
// Enable=true are reported; managed is updated from the labels read here.
func (p *Provider) buildEvent(ctx context.Context, managed map[string]bool, name string, status *dbus.UnitStatus, wantStopped, wantStarted, wantRemoved bool) (sablier.InstanceEvent, bool) {
	if status == nil {
		wasManaged := managed[name]
		delete(managed, name)
		if !wasManaged || !wantRemoved {
			return sablier.InstanceEvent{}, false
		}
		return sablier.InstanceEvent{
			Type: provider.InstanceEventRemoved,
			Info: sablier.InstanceInfo{Name: name, Provider: sablier.ProviderSystemd},
		}, true
	}

	var kind provider.InstanceEventType
	switch {
	case status.LoadState == "not-found":
		return sablier.InstanceEvent{}, false
	case status.ActiveState == "active" && wantStarted:
		kind = provider.InstanceEventStarted
	case slices.Contains([]string{"inactive", "failed", "maintenance"}, status.ActiveState) && wantStopped:
		kind = provider.InstanceEventStopped
	case !wantRemoved:
		// Nothing to report and no removal to track.
		return sablier.InstanceEvent{}, false
	}

	labels, err := p.getLabels(ctx, name)
	if err == nil {
		managed[name] = labels[sablier.LabelEnable] == "true"
	} else if !managed[name] {
		// Whether the unit opted in is unknown. Skip it rather than report a
		// unit that may not be managed.
		p.l.DebugContext(ctx, "cannot read unit file, skipping unit change", slog.String("unit", name), slog.Any("error", err))
		return sablier.InstanceEvent{}, false
	}
	if kind == "" || !managed[name] {
		return sablier.InstanceEvent{}, false
	}

	if err != nil {
		// The unit was managed when last seen; keep reporting it.
		p.l.WarnContext(ctx, "cannot read unit file after unit change, using bare info", slog.String("unit", name), slog.Any("error", err))
		info := sablier.InstanceInfo{Name: name, Provider: sablier.ProviderSystemd, Status: sablier.InstanceStatusStopped}
		if kind == provider.InstanceEventStarted {
			info.Status = sablier.InstanceStatusStarting
		}
		return sablier.InstanceEvent{Type: kind, Info: info}, true
	}
	return sablier.InstanceEvent{Type: kind, Info: p.infoFromUnit(Unit{status: *status, labels: labels})}, true
}
