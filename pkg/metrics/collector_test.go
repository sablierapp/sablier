package metrics_test

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/sablierapp/sablier/pkg/metrics"
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
