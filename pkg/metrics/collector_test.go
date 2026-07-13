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

func TestInstanceGroupCollector_EmitsMembership(t *testing.T) {
	gp := fakeGroupsProvider{groups: map[string][]string{
		"team-a": {"frontend", "shared-api"},
		"team-b": {"shared-api"},
	}}
	c := metrics.NewInstanceGroupCollector(gp)

	want := `
# HELP sablier_instance_group Mapping of instances to the groups they belong to. Always 1. Join with on(instance) group_left(group) to slice per-instance metrics by group.
# TYPE sablier_instance_group gauge
sablier_instance_group{group="team-a",instance="frontend"} 1
sablier_instance_group{group="team-a",instance="shared-api"} 1
sablier_instance_group{group="team-b",instance="shared-api"} 1
`
	if err := testutil.CollectAndCompare(c, strings.NewReader(want), "sablier_instance_group"); err != nil {
		t.Fatalf("CollectAndCompare: %v", err)
	}
}

func TestInstanceGroupCollector_DeduplicatesMembers(t *testing.T) {
	// A duplicate (instance, group) labelset makes Gather() fail; the collector
	// must emit it once.
	gp := fakeGroupsProvider{groups: map[string][]string{
		"team-a": {"frontend", "frontend", "shared-api"},
	}}
	c := metrics.NewInstanceGroupCollector(gp)

	want := `
# HELP sablier_instance_group Mapping of instances to the groups they belong to. Always 1. Join with on(instance) group_left(group) to slice per-instance metrics by group.
# TYPE sablier_instance_group gauge
sablier_instance_group{group="team-a",instance="frontend"} 1
sablier_instance_group{group="team-a",instance="shared-api"} 1
`
	if err := testutil.CollectAndCompare(c, strings.NewReader(want), "sablier_instance_group"); err != nil {
		t.Fatalf("CollectAndCompare: %v", err)
	}
}

func TestInstanceGroupCollector_NoGroupsEmitsNothing(t *testing.T) {
	gp := fakeGroupsProvider{groups: map[string][]string{}}
	c := metrics.NewInstanceGroupCollector(gp)
	if got := testutil.CollectAndCount(c, "sablier_instance_group"); got != 0 {
		t.Errorf("expected 0 series, got %d", got)
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
	for i := 0; i < 3; i++ {
		<-time.After(10 * time.Millisecond)
		if got := testutil.ToFloat64(c); got != first {
			t.Fatalf("expiry moved across scrapes (%v -> %v): the collector renews sessions", first, got)
		}
	}
}
