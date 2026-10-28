package sablier

import (
	"context"
	"log/slog"
	"maps"
	"slices"
	"time"

	"github.com/sablierapp/sablier/pkg/provider"
)

// watchedActivity is the activity configuration of a watched instance,
// resolved when its watch started.
type watchedActivity struct {
	// duration is the effective session duration of a refresh.
	duration time.Duration
	minBytes uint64
	// groups are the groups a refresh extends. Nil means only the instance
	// itself (D6: the implicit default group).
	groups []string
}

// activityWatcher is the state of one WatchActivity loop. It is only used by
// that loop's goroutine, so it needs no lock.
type activityWatcher struct {
	s        *Sablier
	src      ActivitySource
	interval time.Duration
	watched  map[string]watchedActivity
}

// WatchActivity refreshes sessions from instance traffic. Every interval it
// drains the bytes src counted on each watched instance, and an instance that
// carried at least its sablier.activity.min-bytes opens or extends the
// sessions of every member of its groups, like a proxy request would. A
// refresh never shortens a session.
//
// Like GroupWatch, it keeps the watch set in step with the provider through
// started, stopped and removed events, and lists the running instances every
// ActivityReconcileInterval as a safety net for missed events and failed
// watches.
func (s *Sablier) WatchActivity(ctx context.Context, src ActivitySource, interval time.Duration) {
	stream := s.provider.InstanceEvents(ctx, provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{
			provider.InstanceEventStarted,
			provider.InstanceEventStopped,
			provider.InstanceEventRemoved,
		},
	})
	eventsC := stream.Events
	errC := stream.Err

	w := &activityWatcher{s: s, src: src, interval: interval, watched: map[string]watchedActivity{}}

	reconcile := time.NewTicker(s.ActivityReconcileInterval)
	defer reconcile.Stop()
	activity := time.NewTicker(interval)
	defer activity.Stop()

	w.reconcile(ctx)

	for {
		select {
		case <-ctx.Done():
			s.l.InfoContext(ctx, "stop watching activity", slog.Any("reason", ctx.Err()))
			return

		case event, ok := <-eventsC:
			if !ok {
				s.l.WarnContext(ctx, "activity event stream closed; relying on reconciliation ticker")
				eventsC = nil // disable this case
				continue
			}
			w.handleEvent(ctx, event)

		case err, ok := <-errC:
			if !ok {
				errC = nil
				continue
			}
			s.l.ErrorContext(ctx, "activity event stream permanently lost; relying on reconciliation ticker", slog.Any("error", err))
			errC = nil
			eventsC = nil

		case <-reconcile.C:
			w.reconcile(ctx)

		case <-activity.C:
			w.refresh(ctx)
		}
	}
}

func (w *activityWatcher) handleEvent(ctx context.Context, event InstanceEvent) {
	switch event.Type {
	case provider.InstanceEventStarted:
		w.watch(ctx, event.Info)
	case provider.InstanceEventStopped, provider.InstanceEventRemoved:
		w.unwatch(ctx, event.Info.Name)
	}
}

// reconcile watches the running activity-enabled instances that are not
// watched yet, which also retries failed watches, and unwatches the watched
// instances that are no longer running.
func (w *activityWatcher) reconcile(ctx context.Context) {
	instances, err := w.s.provider.InstanceList(ctx, provider.InstanceListOptions{All: false})
	if err != nil {
		w.s.l.ErrorContext(ctx, "activity reconciliation failed to list instances", slog.Any("error", err))
		return
	}

	running := make(map[string]struct{}, len(instances))
	for _, configured := range instances {
		running[configured.Name] = struct{}{}
		// Inspecting is an API call per instance: skip the instances without
		// activity tracking first, they are usually most of the fleet.
		if !configured.IsEnabled() || !configured.Activity {
			continue
		}
		if _, ok := w.watched[configured.Name]; ok {
			continue
		}
		info, err := w.s.provider.InstanceInspect(ctx, configured.Name)
		if err != nil {
			w.s.l.WarnContext(ctx, "activity reconciliation failed to inspect instance", slog.String("instance", configured.Name), slog.Any("error", err))
			continue
		}
		w.watch(ctx, info)
	}

	for name := range w.watched {
		if _, ok := running[name]; !ok {
			w.unwatch(ctx, name)
		}
	}
}

// watch starts watching a running instance when it opted into activity
// tracking. An instance that is already watched is unwatched first: a started
// event for it means it restarted, and its previous watch is stale. A failed
// watch is not recorded, so the next reconciliation retries it.
func (w *activityWatcher) watch(ctx context.Context, info InstanceInfo) {
	w.unwatch(ctx, info.Name)

	if info.Config == nil || !info.Config.Enabled || info.Config.Activity == nil {
		return
	}
	activity := info.Config.Activity

	duration := activity.SessionDuration
	if duration == 0 {
		duration = w.s.DefaultSessionDuration
	}
	// Traffic must refresh a session before it expires, or the instance
	// stops between two ticks while still in use.
	if duration <= w.interval {
		w.s.l.WarnContext(ctx, "not watching instance for activity: its session duration must be longer than the activity interval",
			slog.String("instance", info.Name),
			slog.Duration("session_duration", duration),
			slog.Duration("interval", w.interval),
		)
		return
	}

	if err := w.src.Watch(ctx, info.Name, activity.Ports); err != nil {
		w.s.l.WarnContext(ctx, "cannot watch instance for activity", slog.String("instance", info.Name), slog.Any("error", err))
		return
	}
	w.watched[info.Name] = watchedActivity{
		duration: duration,
		minBytes: activity.MinBytes,
		groups:   activity.Groups,
	}
	w.s.metrics.SetActivityWatched(len(w.watched))
	w.s.l.InfoContext(ctx, "watching instance for activity", slog.String("instance", info.Name), slog.Any("ports", activity.Ports))
}

// unwatch stops watching name, if it is watched.
func (w *activityWatcher) unwatch(ctx context.Context, name string) {
	if _, ok := w.watched[name]; !ok {
		return
	}
	if err := w.src.Unwatch(ctx, name); err != nil {
		w.s.l.WarnContext(ctx, "cannot unwatch instance for activity", slog.String("instance", name), slog.Any("error", err))
	}
	delete(w.watched, name)
	w.s.metrics.SetActivityWatched(len(w.watched))
	w.s.l.InfoContext(ctx, "stopped watching instance for activity", slog.String("instance", name))
}

// refresh drains the traffic counted since the previous tick and refreshes the
// sessions of the active instances' groups.
func (w *activityWatcher) refresh(ctx context.Context) {
	deltas, err := w.src.Drain(ctx)
	if err != nil {
		w.s.l.WarnContext(ctx, "cannot read instance activity", slog.Any("error", err))
		return
	}

	var active []string
	for name, bytes := range deltas {
		w.s.metrics.RecordActivityBytes(name, bytes)
		if cfg, ok := w.watched[name]; ok && bytes >= cfg.minBytes {
			active = append(active, name)
		}
	}
	if len(active) == 0 {
		return
	}

	// A refresh replaces a session's remaining time: read the current
	// expirations so a refresh only ever extends them. Without them, a
	// refresh could cut a longer proxy session short, so skip the tick.
	expiry := map[string]time.Time{}
	err = w.s.sessions.Range(ctx, func(info InstanceInfo, expiresAt time.Time) {
		expiry[info.Name] = expiresAt
	})
	if err != nil {
		w.s.l.WarnContext(ctx, "cannot read sessions, skipping activity refresh", slog.Any("error", err))
		return
	}
	now := time.Now()
	remaining := func(name string) time.Duration {
		if expiresAt, ok := expiry[name]; ok {
			return expiresAt.Sub(now)
		}
		return 0
	}

	// Several active instances can share a group: refresh it once, for the
	// longest of their durations.
	groups := map[string]time.Duration{}
	singles := map[string]time.Duration{}
	for _, name := range active {
		cfg := w.watched[name]
		if cfg.groups == nil {
			singles[name] = cfg.duration
			continue
		}
		for _, g := range cfg.groups {
			groups[g] = max(groups[g], cfg.duration)
		}
	}

	for _, g := range slices.Sorted(maps.Keys(groups)) {
		d := groups[g]
		members, _ := w.s.groups.Get(g)
		for _, m := range members {
			d = max(d, remaining(m))
		}
		w.s.metrics.RecordSessionRequest("activity", g)
		state, err := w.s.RequestSessionGroup(ctx, g, d)
		w.logRefresh(ctx, "group", g, d, state, err)
	}
	for _, name := range slices.Sorted(maps.Keys(singles)) {
		d := max(singles[name], remaining(name))
		w.s.metrics.RecordSessionRequest("activity", name)
		state, err := w.s.RequestSession(ctx, []string{name}, d)
		w.logRefresh(ctx, "instance", name, d, state, err)
	}
}

// logRefresh logs the outcome of one refresh. A failed refresh is retried on
// the next tick if the traffic goes on.
func (w *activityWatcher) logRefresh(ctx context.Context, kind, target string, d time.Duration, state *SessionState, err error) {
	if err == nil && state != nil {
		err = state.InstanceErrors()
	}
	if err != nil {
		w.s.l.WarnContext(ctx, "activity refresh failed", slog.String(kind, target), slog.Duration("duration", d), slog.Any("error", err))
		return
	}
	w.s.l.DebugContext(ctx, "activity refreshed session", slog.String(kind, target), slog.Duration("duration", d))
}
