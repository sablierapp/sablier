package metrics_test

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/sablierapp/sablier/pkg/metrics"
	"github.com/sablierapp/sablier/pkg/sablier"
	"github.com/sablierapp/sablier/pkg/store/inmemory"
)

type fakeGroupsProvider struct {
	groups map[string][]string
}

func (f fakeGroupsProvider) Groups() map[string][]string {
	out := make(map[string][]string, len(f.groups))
	for k, v := range f.groups {
		cp := make([]string, len(v))
		copy(cp, v)
		out[k] = cp
	}
	return out
}

func TestGroupLockCollector_EmitsZeroAndNonZero(t *testing.T) {
	r := metrics.NewPromRecorder()
	r.RecordActiveInstance("web1")

	gp := fakeGroupsProvider{groups: map[string][]string{
		"web":   {"web1", "web2"},
		"empty": {"none1", "none2"},
	}}

	c := metrics.NewGroupLockCollector(gp, r)
	reg := prometheus.NewRegistry()
	reg.MustRegister(c)

	want := `
# HELP sablier_group_active_instances Number of instances in the group with an active session.
# TYPE sablier_group_active_instances gauge
sablier_group_active_instances{group="empty"} 0
sablier_group_active_instances{group="web"} 1
# HELP sablier_group_locked Whether the group has at least one instance with an active session (1) or not (0).
# TYPE sablier_group_locked gauge
sablier_group_locked{group="empty"} 0
sablier_group_locked{group="web"} 1
`

	if err := testutil.CollectAndCompare(c, strings.NewReader(want),
		"sablier_group_active_instances", "sablier_group_locked"); err != nil {
		t.Fatalf("CollectAndCompare: %v", err)
	}
}

func TestGroupLockCollector_NoGroupsEmitsNothing(t *testing.T) {
	r := metrics.NewPromRecorder()
	gp := fakeGroupsProvider{groups: map[string][]string{}}

	c := metrics.NewGroupLockCollector(gp, r)
	got := testutil.CollectAndCount(c, "sablier_group_locked", "sablier_group_active_instances")
	if got != 0 {
		t.Errorf("expected 0 series with no groups, got %d", got)
	}
}

type fakeSessionSource struct {
	entries []metrics.SessionEntry
}

func (f fakeSessionSource) SessionsSnapshot() []metrics.SessionEntry {
	return f.entries
}

func TestSessionExpiryCollector_ExposesGauge(t *testing.T) {
	src := fakeSessionSource{entries: []metrics.SessionEntry{
		{Instance: "whoami", Group: "demo", ExpiresAt: time.Unix(1000000000, 0)},
		{Instance: "nginx", Group: "", ExpiresAt: time.Unix(1000000060, 0)},
	}}

	c := metrics.NewSessionExpiryCollector(src)

	want := `
# HELP sablier_session_expires_at_timestamp_seconds Unix timestamp (seconds) at which the instance's session expires and the instance is stopped. One series per active session. The value tracks the latest access and is pushed back on every session renewal. Derive the remaining time in Grafana with the expression: value - time().
# TYPE sablier_session_expires_at_timestamp_seconds gauge
sablier_session_expires_at_timestamp_seconds{group="",instance="nginx"} 1000000060
sablier_session_expires_at_timestamp_seconds{group="demo",instance="whoami"} 1000000000
`

	if err := testutil.CollectAndCompare(c, strings.NewReader(want),
		"sablier_session_expires_at_timestamp_seconds"); err != nil {
		t.Fatalf("CollectAndCompare: %v", err)
	}

	lints, err := testutil.CollectAndLint(c, "sablier_session_expires_at_timestamp_seconds")
	if err != nil {
		t.Fatalf("CollectAndLint: %v", err)
	}
	if len(lints) > 0 {
		t.Errorf("lint problems: %+v", lints)
	}
}

func TestSessionExpiryCollector_NoSessionsEmitsNothing(t *testing.T) {
	c := metrics.NewSessionExpiryCollector(fakeSessionSource{})
	got := testutil.CollectAndCount(c, "sablier_session_expires_at_timestamp_seconds")
	if got != 0 {
		t.Errorf("expected 0 series with no sessions, got %d", got)
	}
}

// TestSessionExpiryCollector_NonDestructive exercises the full production path
// (store -> Sablier.SessionsSnapshot -> collector) and proves that scraping the
// metric never renews the session: the exposed expiry must stay constant across
// repeated scrapes.
func TestSessionExpiryCollector_NonDestructive(t *testing.T) {
	ctx := context.Background()
	st := inmemory.NewInMemory()
	if err := st.Put(ctx, sablier.InstanceInfo{Name: "whoami", Groups: []string{"demo"}}, 5*time.Minute); err != nil {
		t.Fatalf("seed store: %v", err)
	}

	s := sablier.New(slog.New(slog.NewTextHandler(io.Discard, nil)), st, nil)
	c := metrics.NewSessionExpiryCollector(s)

	first := testutil.ToFloat64(c)
	if first <= 0 {
		t.Fatalf("expected a positive expiry timestamp, got %v", first)
	}
	for range 3 {
		<-time.After(10 * time.Millisecond)
		if got := testutil.ToFloat64(c); got != first {
			t.Fatalf("expiry moved across scrapes (%v -> %v): the collector renews sessions", first, got)
		}
	}
}

func TestSessionStartupCollector_ExposesStartsInProgress(t *testing.T) {
	src := fakeSessionSource{entries: []metrics.SessionEntry{
		// Starting, with a learned startup.
		{Instance: "opencell", Group: "demo", ExpiresAt: time.Unix(1000000600, 0), StartingSince: time.Unix(1000000000, 0), TypicalStartup: 6 * time.Minute},
		// Starting for the first time since Sablier started: nothing learned yet.
		{Instance: "whoami", Group: "", ExpiresAt: time.Unix(1000000600, 0), StartingSince: time.Unix(1000000030, 0)},
		// Ready: no start in progress.
		{Instance: "nginx", Group: "", ExpiresAt: time.Unix(1000000600, 0)},
	}}

	c := metrics.NewSessionStartupCollector(src)

	want := `
# HELP sablier_instance_typical_startup_seconds Median time (seconds) the instance took to become ready over its last observed starts, learned in memory since Sablier started. Emitted alongside sablier_session_starting_since_timestamp_seconds, once one start of the instance has been observed.
# TYPE sablier_instance_typical_startup_seconds gauge
sablier_instance_typical_startup_seconds{group="demo",instance="opencell"} 360
# HELP sablier_session_starting_since_timestamp_seconds Unix timestamp (seconds) at which Sablier started the instance for its current session. One series per session whose instance is not ready yet; it disappears once a request sees the instance ready. Derive how long the start has lasted with the expression: time() - value.
# TYPE sablier_session_starting_since_timestamp_seconds gauge
sablier_session_starting_since_timestamp_seconds{group="",instance="whoami"} 1000000030
sablier_session_starting_since_timestamp_seconds{group="demo",instance="opencell"} 1000000000
`

	if err := testutil.CollectAndCompare(c, strings.NewReader(want)); err != nil {
		t.Fatalf("CollectAndCompare: %v", err)
	}

	lints, err := testutil.CollectAndLint(c)
	if err != nil {
		t.Fatalf("CollectAndLint: %v", err)
	}
	if len(lints) > 0 {
		t.Errorf("lint problems: %+v", lints)
	}
}

// TestSessionStartupCollector_FromStore exercises the production path (store ->
// Sablier.SessionsSnapshot -> collector): only sessions whose instance is not
// ready yet expose when their start began.
func TestSessionStartupCollector_FromStore(t *testing.T) {
	ctx := context.Background()
	st := inmemory.NewInMemory()
	startedAt := time.Unix(1000000000, 0)
	for _, info := range []sablier.InstanceInfo{
		{Name: "starting", Status: sablier.InstanceStatusStarting, StartedAt: &startedAt},
		{Name: "ready", Status: sablier.InstanceStatusReady, StartedAt: &startedAt},
		{Name: "warm", Status: sablier.InstanceStatusReady},
	} {
		if err := st.Put(ctx, info, 5*time.Minute); err != nil {
			t.Fatalf("seed store: %v", err)
		}
	}

	s := sablier.New(slog.New(slog.NewTextHandler(io.Discard, nil)), st, nil)
	c := metrics.NewSessionStartupCollector(s)

	want := `
# HELP sablier_session_starting_since_timestamp_seconds Unix timestamp (seconds) at which Sablier started the instance for its current session. One series per session whose instance is not ready yet; it disappears once a request sees the instance ready. Derive how long the start has lasted with the expression: time() - value.
# TYPE sablier_session_starting_since_timestamp_seconds gauge
sablier_session_starting_since_timestamp_seconds{group="",instance="starting"} 1000000000
`
	if err := testutil.CollectAndCompare(c, strings.NewReader(want)); err != nil {
		t.Fatalf("CollectAndCompare: %v", err)
	}
}
