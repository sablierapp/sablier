package sablier

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/go-cmp/cmp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	"github.com/sablierapp/sablier/pkg/metrics"
)

type Sablier struct {
	provider Provider
	sessions Store

	groups *groupRegistry

	pendingMu     sync.Mutex
	pendingStarts map[string]*pendingStart

	// BlockingRefreshFrequency is the frequency at which the instances are checked
	// against the provider. Defaults to 5 seconds.
	BlockingRefreshFrequency time.Duration

	// InstanceStartTimeout is the maximum time allowed for an async InstanceStart
	// call before it is cancelled. Defaults to 5 minutes.
	InstanceStartTimeout time.Duration

	// ExternallyStartedScanInterval is how often WatchAndStopExternallyStarted and
	// WatchAndWarmExternallyStarted perform a full reconciliation scan. Defaults to 30 seconds.
	ExternallyStartedScanInterval time.Duration

	// DefaultSessionDuration is the session duration used when seeding sessions
	// for externally started instances (WatchAndWarmExternallyStarted).
	// Defaults to 5 minutes.
	DefaultSessionDuration time.Duration

	// RunningHoursRefreshFrequency is how often running-hours windows are
	// reconciled. Defaults to 30 seconds.
	RunningHoursRefreshFrequency time.Duration

	// rejectUnlabeledRequests blocks direct named requests unless sablier.enable=true.
	rejectUnlabeledRequests bool

	// verifyEnabledOnExpiration re-checks sablier.enable before stopping expired instances.
	verifyEnabledOnExpiration bool

	metrics metrics.Recorder
	tracer  trace.Tracer

	l *slog.Logger
}

func New(logger *slog.Logger, store Store, provider Provider) *Sablier {
	return &Sablier{
		provider:                      provider,
		sessions:                      store,
		groups:                        newGroupRegistry(),
		pendingStarts:                 map[string]*pendingStart{},
		l:                             logger,
		metrics:                       metrics.Noop{},
		tracer:                        otel.Tracer("github.com/sablierapp/sablier"),
		BlockingRefreshFrequency:      5 * time.Second,
		InstanceStartTimeout:          5 * time.Minute,
		ExternallyStartedScanInterval: 30 * time.Second,
		DefaultSessionDuration:        5 * time.Minute,
		RunningHoursRefreshFrequency:  30 * time.Second,
	}
}

// WithRejectUnlabeledRequests makes direct named requests require sablier.enable=true.
func (s *Sablier) WithRejectUnlabeledRequests(reject bool) {
	s.rejectUnlabeledRequests = reject
}

// WithVerifyEnabledOnExpiration makes Sablier re-check sablier.enable before stopping expired instances.
func (s *Sablier) WithVerifyEnabledOnExpiration(verify bool) {
	s.verifyEnabledOnExpiration = verify
}

// WithMetrics installs a Recorder. Defaults to metrics.Noop until called.
func (s *Sablier) WithMetrics(r metrics.Recorder) {
	if r == nil {
		r = metrics.Noop{}
	}
	s.metrics = r
}

// Groups returns a defensive copy of the current group→instances map.
// Safe for concurrent use; intended for the metrics GroupLockCollector.
func (s *Sablier) Groups() map[string][]string {
	return s.groups.Snapshot()
}

// SetGroups replaces the entire group registry. Changes are logged with a diff.
func (s *Sablier) SetGroups(groups map[string][]string) {
	old, changed := s.groups.Set(groups)
	if changed {
		s.l.Info("set groups",
			slog.Any("old", old),
			slog.Any("new", groups),
			slog.Any("diff", cmp.Diff(old, groups)),
		)
	}
}

// GroupsForInstance returns all groups the instance currently belongs to.
func (s *Sablier) GroupsForInstance(name string) []string {
	return s.groups.GroupsOf(name)
}

// SyncInstanceGroups sets the instance's group memberships to exactly newGroups,
// adding and removing as needed. Returns the added and removed group names.
func (s *Sablier) SyncInstanceGroups(instance string, newGroups []string) (added, removed []string) {
	return s.groups.Sync(instance, newGroups)
}

// RemoveInstanceFromAllGroups removes instance from all groups it belongs to.
// Returns the list of groups it was removed from.
func (s *Sablier) RemoveInstanceFromAllGroups(instance string) []string {
	return s.groups.Remove(instance)
}

func (s *Sablier) RemoveInstance(ctx context.Context, name string) error {
	return s.sessions.Delete(ctx, name)
}
