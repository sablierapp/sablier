package sablier

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/store"
)

// WarmAllUnregisteredInstances creates a session for all auto-discovered running
// instances that are not yet registered as running instances by Sablier.
// This is the non-destructive counterpart to StopAllUnregisteredInstances: instead
// of stopping an externally started instance, it seeds a session with the default
// session duration so the regular expiration lifecycle takes over.
func (s *Sablier) WarmAllUnregisteredInstances(ctx context.Context) error {
	instances, err := s.provider.InstanceList(ctx, provider.InstanceListOptions{
		All: false, // Only running instances
	})
	if err != nil {
		return err
	}

	unregistered := make([]string, 0)
	for _, instance := range instances {
		if !s.isStartedByUs(ctx, instance.Name) {
			unregistered = append(unregistered, instance.Name)
		}
	}

	s.l.DebugContext(ctx, "found instances to warm", slog.Any("instances", unregistered))

	for _, name := range unregistered {
		s.seedSession(ctx, name)
	}

	return nil
}

// seedSession registers a session with the default session duration for an
// already-running instance, without going through the start path.
// It is a no-op when the instance already has a session (so an existing session
// is never renewed by the watch loop), when it cannot be inspected, or when it
// is not ready yet (the reconciliation scan will pick it up once ready).
func (s *Sablier) seedSession(ctx context.Context, name string) {
	_, err := s.sessions.Get(ctx, name)
	if err == nil {
		return // the instance already has a session
	}
	if !errors.Is(err, store.ErrKeyNotFound) {
		s.l.WarnContext(ctx, "session lookup failed, not warming instance", slog.String("instance", name), slog.Any("error", err))
		return
	}

	info, err := s.provider.InstanceInspect(ctx, name)
	if err != nil {
		s.l.WarnContext(ctx, "instance inspect failed, not warming instance", slog.String("instance", name), slog.Any("error", err))
		return
	}
	if info.Status != InstanceStatusReady {
		return
	}

	if err := s.sessions.Put(ctx, info, s.DefaultSessionDuration); err != nil {
		s.l.WarnContext(ctx, "failed to seed session for instance", slog.String("instance", name), slog.Any("error", err))
		return
	}
	s.l.InfoContext(ctx, "seeded session for externally started instance",
		slog.String("instance", name),
		slog.Duration("duration", s.DefaultSessionDuration),
		slog.String("reason", "instance is enabled but not started by Sablier"))
}

// WatchAndWarmExternallyStarted continuously creates sessions for instances that
// have sablier.enable=true but were not started by Sablier. It combines
// event-driven detection (InstanceEventStarted) with a periodic
// reconciliation ticker as a safety net.
//
// This is the non-destructive counterpart to WatchAndStopExternallyStarted:
// instead of stopping an externally started instance, it seeds a session with
// the default session duration. The instance then hibernates through the
// regular expiration lifecycle if it stays idle. Call it in a dedicated goroutine.
func (s *Sablier) WatchAndWarmExternallyStarted(ctx context.Context) {
	stream := s.provider.InstanceEvents(ctx, provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventStarted},
	})
	eventsC := stream.Events
	errC := stream.Err

	ticker := time.NewTicker(s.ExternallyStartedScanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.l.InfoContext(ctx, "stop watching for unregistered instances to warm", slog.Any("reason", ctx.Err()))
			return
		case info, ok := <-eventsC:
			if !ok {
				s.l.WarnContext(ctx, "started event stream closed; relying on reconciliation ticker")
				eventsC = nil // disable this select case
				continue
			}
			// Only act on Sablier-managed instances.
			if !info.Info.IsEnabled() {
				continue
			}
			if s.isStartedByUs(ctx, info.Info.Name) {
				s.l.DebugContext(ctx, "instance started by Sablier, skipping", slog.String("instance", info.Info.Name))
				continue
			}
			s.l.InfoContext(ctx, "externally started instance detected, warming", slog.String("instance", info.Info.Name))
			s.seedSession(ctx, info.Info.Name)
		case err, ok := <-errC:
			if !ok {
				errC = nil // disable this select case
				continue
			}
			s.l.ErrorContext(ctx, "started event stream permanently lost; relying on reconciliation ticker", slog.Any("error", err))
			errC = nil
			eventsC = nil
		case <-ticker.C:
			if err := s.WarmAllUnregisteredInstances(ctx); err != nil {
				s.l.ErrorContext(ctx, "unregistered instance warm scan failed", slog.Any("error", err))
			}
		}
	}
}
