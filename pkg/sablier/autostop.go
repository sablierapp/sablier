package sablier

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/store"
	"golang.org/x/sync/errgroup"
)

// StopAllUnregisteredInstances stops all auto-discovered running instances that are not yet registered
// as running instances by Sablier.
// By default, Sablier does not stop all already running instances. Meaning that you need to make an
// initial request in order to trigger the scaling to zero.
func (s *Sablier) StopAllUnregisteredInstances(ctx context.Context) error {
	instances, err := s.provider.InstanceList(ctx, provider.InstanceListOptions{
		All: false, // Only running instances
	})
	if err != nil {
		return err
	}

	unregistered := make([]string, 0)
	for _, instance := range instances {
		_, err = s.sessions.Get(ctx, instance.Name)
		if errors.Is(err, store.ErrKeyNotFound) {
			unregistered = append(unregistered, instance.Name)
		}
	}

	s.l.DebugContext(ctx, "found instances to stop", slog.Any("instances", unregistered))

	waitGroup := errgroup.Group{}

	for _, name := range unregistered {
		waitGroup.Go(s.stopFunc(ctx, name))
	}

	return waitGroup.Wait()
}

func (s *Sablier) stopFunc(ctx context.Context, name string) func() error {
	return func() error {
		err := s.provider.InstanceStop(ctx, name)
		if err != nil {
			s.l.ErrorContext(ctx, "failed to stop instance", slog.String("instance", name), slog.Any("error", err))
			return err
		}
		s.metrics.RecordInstanceStop(name, "unregistered")
		s.l.InfoContext(ctx, "stopped unregistered instance", slog.String("instance", name), slog.String("reason", "instance is enabled but not started by Sablier"))
		return nil
	}
}

// isStartedByUs returns true if the instance was initiated by Sablier:
// either it has an in-progress start goroutine (pendingStarts) or it is
// already registered in the sessions store.
// Both checks are needed to cover all timing windows between when Sablier
// triggers a start and when the session entry is written.
func (s *Sablier) isStartedByUs(ctx context.Context, name string) bool {
	s.pendingMu.Lock()
	_, pending := s.pendingStarts[name]
	s.pendingMu.Unlock()
	if pending {
		return true
	}
	_, err := s.sessions.Get(ctx, name)
	return err == nil
}

// WatchAndStopExternallyStarted continuously stops instances that have
// sablier.enable=true but were not started by Sablier. It combines
// event-driven detection (InstanceEventStarted) with a periodic
// reconciliation ticker as a safety net.
//
// This is the continuous counterpart to StopAllUnregisteredInstances, which
// only runs once at startup. Call it in a dedicated goroutine.
func (s *Sablier) WatchAndStopExternallyStarted(ctx context.Context) {
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
			s.l.InfoContext(ctx, "stop watching for unregistered instances", slog.Any("reason", ctx.Err()))
			return
		case info, ok := <-eventsC:
			if !ok {
				s.l.WarnContext(ctx, "started event stream closed; relying on reconciliation ticker")
				eventsC = nil // disable this select case
				continue
			}
			// Only act on Sablier-managed instances.
			if info.Info.Enabled != "true" {
				continue
			}
			if s.isStartedByUs(ctx, info.Info.Name) {
				s.l.DebugContext(ctx, "instance started by Sablier, skipping", slog.String("instance", info.Info.Name))
				continue
			}
			s.l.InfoContext(ctx, "externally started instance detected, stopping", slog.String("instance", info.Info.Name))
			if err := s.provider.InstanceStop(ctx, info.Info.Name); err != nil {
				s.l.ErrorContext(ctx, "failed to stop externally-started instance", slog.String("instance", info.Info.Name), slog.Any("error", err))
			} else {
				s.metrics.RecordInstanceStop(info.Info.Name, "externally-started")
			}
		case err, ok := <-errC:
			if !ok {
				errC = nil // disable this select case
				continue
			}
			s.l.ErrorContext(ctx, "started event stream permanently lost; relying on reconciliation ticker", slog.Any("error", err))
			errC = nil
			eventsC = nil
		case <-ticker.C:
			if err := s.StopAllUnregisteredInstances(ctx); err != nil {
				s.l.ErrorContext(ctx, "unregistered instance scan failed", slog.Any("error", err))
			}
		}
	}
}
