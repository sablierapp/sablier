package sablier

import (
	"context"
	"errors"
	"log/slog"
	"sync"
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
		if s.isUnregistered(ctx, instance.Name) {
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
		// The list and the stops are separate steps: a request may have started
		// the instance since the scan looked at it.
		if !s.isUnregistered(ctx, name) {
			s.l.DebugContext(ctx, "instance started by Sablier since the scan, skipping", slog.String("instance", name))
			return nil
		}
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

// isUnregistered returns true if Sablier has no trace of the instance: no
// start in progress (pendingStarts) and no entry in the sessions store.
// A store error other than "not found" is not enough to call it unregistered.
func (s *Sablier) isUnregistered(ctx context.Context, name string) bool {
	if s.isPendingStart(name) {
		return false
	}
	_, err := s.sessions.Get(ctx, name)
	return errors.Is(err, store.ErrKeyNotFound)
}

func (s *Sablier) isPendingStart(name string) bool {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	_, pending := s.pendingStarts[name]
	return pending
}

// isStartedByUs returns true if the instance was initiated by Sablier:
// either it has an in-progress start goroutine (pendingStarts) or it is
// already registered in the sessions store.
// Both checks are needed to cover all timing windows between when Sablier
// triggers a start and when the session entry is written.
func (s *Sablier) isStartedByUs(ctx context.Context, name string) bool {
	if s.isPendingStart(name) {
		return true
	}
	_, err := s.sessions.Get(ctx, name)
	return err == nil
}

// scanPeriodically runs scan every ExternallyStartedScanInterval until ctx is
// done. WatchAndStopExternallyStarted and WatchAndWarmExternallyStarted run it
// apart from their event loop: a scan inspects instances one by one and can
// take minutes on a large fleet, while the event stream must keep being read.
// Providers send events from their watchers, which stay blocked until the
// event is read.
func (s *Sablier) scanPeriodically(ctx context.Context, scan func(context.Context) error, failure string) {
	ticker := time.NewTicker(s.ExternallyStartedScanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := scan(ctx); err != nil {
				s.l.ErrorContext(ctx, failure, slog.Any("error", err))
			}
		}
	}
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

	var scans sync.WaitGroup
	defer scans.Wait()
	scans.Go(func() {
		s.scanPeriodically(ctx, s.StopAllUnregisteredInstances, "unregistered instance scan failed")
	})

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
			if !info.Info.IsEnabled() {
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
		}
	}
}
