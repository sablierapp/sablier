package systemd

import (
	"context"
	"log/slog"
	"slices"

	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
)

type eventFilters struct {
	stopped, started, created, removed bool
}

func newEventFilters(opts provider.InstanceEventsOptions) eventFilters {
	return eventFilters{
		stopped: len(opts.Types) == 0 || slices.Contains(opts.Types, provider.InstanceEventStopped),
		started: len(opts.Types) == 0 || slices.Contains(opts.Types, provider.InstanceEventStarted),
		created: len(opts.Types) == 0 || slices.Contains(opts.Types, provider.InstanceEventCreated),
		removed: len(opts.Types) == 0 || slices.Contains(opts.Types, provider.InstanceEventRemoved),
	}
}

func (p *Provider) InstanceEvents(ctx context.Context, opts provider.InstanceEventsOptions) sablier.InstanceEventStream {
	filters := newEventFilters(opts)

	eventsC := make(chan sablier.InstanceEvent)
	errC := make(chan error, 1)

	go func() {
		defer close(eventsC)
		defer close(errC)

		msgs, errs := p.Con.SubscribeUnitsContext(ctx, p.pollInterval)
		lastState := make(map[string]lifecycleState)
		baseline := true

		for {
			select {
			case changed, ok := <-msgs:
				if !ok {
					p.l.WarnContext(ctx, "event stream closed")
					return
				}
				if !p.handleBatch(ctx, changed, lastState, baseline, filters, eventsC) {
					return
				}
				baseline = false
			case err, ok := <-errs:
				if !ok {
					p.l.WarnContext(ctx, "event stream closed")
					return
				}
				p.l.WarnContext(ctx, "systemd event stream error", slog.Any("error", err))
			case <-ctx.Done():
				for msgs != nil || errs != nil {
					select {
					case _, ok := <-msgs:
						if !ok {
							msgs = nil
						}
					case _, ok := <-errs:
						if !ok {
							errs = nil
						}
					}
				}
				return
			}
		}
	}()

	return sablier.InstanceEventStream{Events: eventsC, Err: errC}
}

// handleBatch processes one poll result and emits the corresponding events.
// It returns false when the stream must shut down.
func (p *Provider) handleBatch(
	ctx context.Context,
	changed map[string]*dbus.UnitStatus,
	lastState map[string]lifecycleState,
	baseline bool,
	filters eventFilters,
	eventsC chan<- sablier.InstanceEvent,
) bool {
	for name, status := range changed {
		if status == nil {
			if !p.handleRemoved(ctx, name, lastState, filters, eventsC) {
				return false
			}
			continue
		}

		kinds := classifyChange(name, status.ActiveState, lastState, filters, baseline)
		for _, kind := range kinds {
			if !p.sendEvent(ctx, name, kind, eventsC) {
				return false
			}
		}
	}
	return true
}

// handleRemoved verifies a vanished unit is truly gone before emitting a
// removed event; systemd unloads inactive units, which also removes them
// from the listing.
func (p *Provider) handleRemoved(
	ctx context.Context,
	name string,
	lastState map[string]lifecycleState,
	filters eventFilters,
	eventsC chan<- sablier.InstanceEvent,
) bool {
	previous, known := lastState[name]
	delete(lastState, name)

	if known && previous == lifecycleRunning && filters.stopped {
		if !p.sendEvent(ctx, name, provider.InstanceEventStopped, eventsC) {
			return false
		}
	}
	if !filters.removed {
		return true
	}

	exists, err := p.unitFileExists(ctx, name)
	if err != nil {
		p.l.WarnContext(ctx, "cannot check unit file existence, skipping removed event", slog.String("unit", name), slog.Any("error", err))
		return true
	}
	if exists {
		return true
	}
	select {
	case eventsC <- sablier.InstanceEvent{Type: provider.InstanceEventRemoved, Info: sablier.InstanceInfo{Name: name, Provider: sablier.ProviderSystemd}}:
	case <-ctx.Done():
		return false
	}
	return true
}

func (p *Provider) unitFileExists(ctx context.Context, name string) (bool, error) {
	files, err := p.Con.ListUnitFilesContext(ctx)
	if err != nil {
		return false, err
	}
	for _, f := range files {
		if unitNameFromPath(f.Path) == name {
			return true, nil
		}
	}
	return false, nil
}

// sendEvent inspects the unit and emits an event of the given type, falling
// back to bare info when the inspect fails.
func (p *Provider) sendEvent(ctx context.Context, name string, kind provider.InstanceEventType, eventsC chan<- sablier.InstanceEvent) bool {
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
	select {
	case eventsC <- sablier.InstanceEvent{Type: kind, Info: info}:
		return true
	case <-ctx.Done():
		return false
	}
}

type lifecycleState uint8

const (
	lifecycleUnknown lifecycleState = iota
	lifecycleStopped
	lifecycleRunning
)

func normalizeLifecycleState(activeState string) lifecycleState {
	switch activeState {
	case "active":
		return lifecycleRunning
	case "inactive", "failed", "maintenance":
		return lifecycleStopped
	default:
		return lifecycleUnknown
	}
}

func classifyChange(
	name string,
	activeState string,
	lastState map[string]lifecycleState,
	filters eventFilters,
	baseline bool,
) []provider.InstanceEventType {
	prev, known := lastState[name]
	next := normalizeLifecycleState(activeState)
	if baseline {
		lastState[name] = next
		if activeState == "active" && filters.started {
			return []provider.InstanceEventType{provider.InstanceEventStarted}
		}
		return nil
	}

	if next == lifecycleUnknown {
		return nil
	}
	lastState[name] = next

	var kinds []provider.InstanceEventType

	if !known && !baseline && filters.created {
		kinds = append(kinds, provider.InstanceEventCreated)
	}

	if next == lifecycleRunning && filters.started && (!known || prev != lifecycleRunning) {
		kinds = append(kinds, provider.InstanceEventStarted)
	}
	if next == lifecycleStopped && filters.stopped && known && prev != lifecycleStopped {
		kinds = append(kinds, provider.InstanceEventStopped)
	}
	return kinds
}
