package config

import "time"

// Activity holds the eBPF activity tracking configuration: traffic on the
// ports of instances labeled sablier.activity.enable=true opens or extends
// their sessions.
type Activity struct {
	// Enabled loads the eBPF programs that count instance traffic. It needs
	// Linux on amd64 or arm64 with cgroup v2, CAP_BPF and CAP_NET_ADMIN, the
	// host cgroup namespace, and the Docker provider. Without the host cgroup
	// namespace, Sablier starts but can't watch any instance, and logs a
	// warning for each; when anything else is missing, it refuses to start.
	// Env: SABLIER_ACTIVITY_ENABLED
	// CLI: --activity.enabled
	// Default: false
	// Since: NEXT_RELEASE
	Enabled bool

	// Interval is how often the traffic counters are read. It must be greater
	// than 0, and an instance's activity session duration must be longer.
	// Env: SABLIER_ACTIVITY_INTERVAL
	// CLI: --activity.interval
	// Default: 20s
	// Since: NEXT_RELEASE
	Interval time.Duration

	// IgnoreCIDRs lists the source address ranges whose traffic never counts
	// as activity, for clients that are not containers, such as a poller on
	// the host network.
	// Env: SABLIER_ACTIVITY_IGNORE_CIDRS
	// CLI: --activity.ignore-cidrs
	// Default: [] (none)
	// Since: NEXT_RELEASE
	IgnoreCIDRs []string
}

func NewActivityConfig() Activity {
	return Activity{
		Enabled:  false,
		Interval: 20 * time.Second,
	}
}
