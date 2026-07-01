package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// GroupsProvider exposes the current configured groups (group name -> instance names).
type GroupsProvider interface {
	Groups() map[string][]string
}

type activeSetProvider interface {
	SnapshotActiveInstances() map[string]struct{}
}

// GroupLockCollector emits group-level gauges lazily at scrape time.
type GroupLockCollector struct {
	groups GroupsProvider
	active activeSetProvider

	lockedDesc *prometheus.Desc
	countDesc  *prometheus.Desc
}

// NewGroupLockCollector wires a GroupsProvider and an activeSetProvider into
// a Collector that can be registered on the same registry as the rest of
// the metrics.
func NewGroupLockCollector(groups GroupsProvider, active activeSetProvider) *GroupLockCollector {
	return &GroupLockCollector{
		groups: groups,
		active: active,
		lockedDesc: prometheus.NewDesc(
			"sablier_group_locked",
			"Whether the group has at least one instance with an active session (1) or not (0).",
			[]string{"group"}, nil,
		),
		countDesc: prometheus.NewDesc(
			"sablier_group_active_instances",
			"Number of instances in the group with an active session.",
			[]string{"group"}, nil,
		),
	}
}

func (c *GroupLockCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.lockedDesc
	ch <- c.countDesc
}

func (c *GroupLockCollector) Collect(ch chan<- prometheus.Metric) {
	groups := c.groups.Groups()
	active := c.active.SnapshotActiveInstances()

	for group, members := range groups {
		count := 0
		for _, m := range members {
			if _, ok := active[m]; ok {
				count++
			}
		}
		locked := 0.0
		if count > 0 {
			locked = 1.0
		}
		ch <- prometheus.MustNewConstMetric(c.lockedDesc, prometheus.GaugeValue, locked, group)
		ch <- prometheus.MustNewConstMetric(c.countDesc, prometheus.GaugeValue, float64(count), group)
	}
}

// SessionEntry is the minimal view of a single active session that the
// SessionExpiryCollector needs to emit its gauge.
type SessionEntry struct {
	Instance  string
	Group     string
	ExpiresAt time.Time
}

// sessionSource enumerates the currently active sessions. Implementations must
// be non-destructive: producing the snapshot must never renew a session.
type sessionSource interface {
	SessionsSnapshot() []SessionEntry
}

// SessionExpiryCollector emits one gauge per active session carrying the unix
// timestamp (seconds) at which the session expires. It reads the session source
// lazily at scrape time and never writes back, so scraping /metrics does not
// renew any session.
type SessionExpiryCollector struct {
	sessions sessionSource

	expiresAtDesc *prometheus.Desc
}

// NewSessionExpiryCollector wires a sessionSource into a Collector that can be
// registered on the same registry as the rest of the metrics.
func NewSessionExpiryCollector(sessions sessionSource) *SessionExpiryCollector {
	return &SessionExpiryCollector{
		sessions: sessions,
		expiresAtDesc: prometheus.NewDesc(
			"sablier_session_expires_at_timestamp_seconds",
			"Unix timestamp (seconds) at which the instance's session expires and the instance is stopped. One series per active session. The value tracks the latest access and is pushed back on every session renewal. Derive the remaining time in Grafana with the expression: value - time().",
			[]string{"instance", "group"}, nil,
		),
	}
}

func (c *SessionExpiryCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.expiresAtDesc
}

func (c *SessionExpiryCollector) Collect(ch chan<- prometheus.Metric) {
	for _, s := range c.sessions.SessionsSnapshot() {
		ch <- prometheus.MustNewConstMetric(
			c.expiresAtDesc,
			prometheus.GaugeValue,
			float64(s.ExpiresAt.Unix()),
			s.Instance, s.Group,
		)
	}
}
