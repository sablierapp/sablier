package sablier

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// DefaultActivityMinBytes is the default for sablier.activity.min-bytes: the
// number of bytes an instance must exchange per --activity.interval, both
// directions together, before it counts as activity.
const DefaultActivityMinBytes uint64 = 64 * 1024

// Port is a transport port watched for activity.
type Port struct {
	Number uint16 `json:"number"`
	Proto  string `json:"proto"` // "tcp" or "udp"
}

// ActivityConfig is the parsed sablier.activity.* configuration of an
// instance that opted into activity tracking.
type ActivityConfig struct {
	Ports           []Port        `json:"ports,omitempty"`           // nil: the provider's exposed TCP and UDP ports
	SessionDuration time.Duration `json:"sessionDuration,omitempty"` // 0: --sessions.default-duration
	MinBytes        uint64        `json:"minBytes"`
	Groups          []string      `json:"groups,omitempty"` // nil: refresh only this instance
}

// ActivitySource counts the traffic of watched instances. It is the core's
// only dependency on activity tracking; WatchActivity drives it.
type ActivitySource interface {
	// Watch starts counting the traffic on the instance's ports. Watching an
	// instance that is already watched replaces its previous watch.
	Watch(ctx context.Context, instance string, ports []Port) error
	// Unwatch stops counting the instance's traffic.
	Unwatch(ctx context.Context, instance string) error
	// Stale returns the watched instances whose watch no longer counts their
	// traffic, such as an instance that restarted while its events were
	// missed. It may return instances along with an error.
	Stale(ctx context.Context) ([]string, error)
	// Drain returns the bytes counted per watched instance since the
	// previous call. It only lists the instances that had traffic: an
	// instance without traffic is left out, never reported with zero bytes.
	Drain(ctx context.Context) (map[string]uint64, error)
}

// ParseActivityPorts parses a comma-separated list of "port[/proto]" entries.
// A bare port means tcp; the protocol is case-insensitive and must be tcp or
// udp; the port must be within 1-65535.
func ParseActivityPorts(v string) ([]Port, error) {
	var ports []Port
	for _, entry := range strings.Split(v, ",") {
		entry = strings.TrimSpace(entry)
		num, proto, _ := strings.Cut(entry, "/")
		proto = strings.ToLower(strings.TrimSpace(proto))
		if proto == "" {
			proto = "tcp"
		}
		if proto != "tcp" && proto != "udp" {
			return nil, fmt.Errorf("invalid protocol %q in port %q: must be tcp or udp", proto, entry)
		}
		n, err := strconv.ParseUint(strings.TrimSpace(num), 10, 16)
		if err != nil || n == 0 {
			return nil, fmt.Errorf("invalid port %q: must be a number between 1 and 65535", entry)
		}
		ports = append(ports, Port{Number: uint16(n), Proto: proto})
	}
	return ports, nil
}
