package sablier

import (
	"context"
	"log/slog"
)

// OnInstanceExpired returns a store-expiration callback that idles the expired
// instance and records the corresponding metrics.
//
// A single InstanceInspect serves both the optional enabled-verification and the
// delegated-scaling detection that selects how the instance is stopped: a
// delegated instance is deactivated via an intent webhook (an external scaler
// owns its replica count) while every other instance is scaled to zero through
// the provider. The expired session may also have been the last one keeping a
// group active, so an anti-affinity reconcile runs afterwards to restore any
// instance held idle because of it.
func (s *Sablier) OnInstanceExpired(ctx context.Context) func(string) {
	return func(_key string) {
		go func(key string) {
			s.l.InfoContext(ctx, "instance expired", slog.String("instance", key))

			// A single inspect serves both the optional enabled-verification and the
			// delegated-scaling detection that selects how the instance is stopped.
			info, err := s.provider.InstanceInspect(ctx, key)
			if err != nil {
				if s.verifyEnabledOnExpiration {
					s.l.WarnContext(ctx, "instance expired could not be inspected before stop", slog.String("instance", key), slog.Any("error", err))
					return
				}
				// Best-effort: without verification we still stop, via a bare
				// (non-delegated) provider stop.
				s.l.WarnContext(ctx, "instance expired could not be inspected, stopping via provider", slog.String("instance", key), slog.Any("error", err))
				info = InstanceInfo{Name: key}
			} else if s.verifyEnabledOnExpiration && !info.IsEnabled() {
				s.l.WarnContext(ctx, "instance expired but is not managed by sablier, skipping stop", slog.String("instance", key))
				return
			}

			if err := s.stop(ctx, key, info); err != nil {
				s.l.ErrorContext(ctx, "instance expired could not be stopped", slog.String("instance", key), slog.Any("error", err))
			}
			s.metrics.RecordInstanceStop(key, "expired")
			s.metrics.RecordInactiveInstance(key)
			s.metrics.DiscardReadyWait(key)
		}(_key)

		// The expired session may have been the last one keeping a group active;
		// restore any instance we forced idle because of an anti-affinity to it.
		// The store key is already gone by the time this callback runs, so the
		// reconcile sees the group as inactive.
		s.triggerAntiAffinityReconcile(ctx)
	}
}
