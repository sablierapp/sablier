// Package local is the single-node sablier.ActivitySource: it maps instances
// to eBPF tracker slots and turns the tracker's running totals into deltas.
package local

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"slices"
	"sync"

	"github.com/sablierapp/sablier/pkg/activity/bpf"
	"github.com/sablierapp/sablier/pkg/sablier"
)

// ErrNoPorts is returned when an instance has neither configured nor exposed
// TCP or UDP ports.
var ErrNoPorts = errors.New("no ports to watch: set sablier.activity.ports or expose a TCP or UDP port")

// Tracker is the part of the eBPF tracker the Source needs.
type Tracker interface {
	Watch(cgroupPath string, ports []bpf.Port) (bpf.Slot, error)
	Unwatch(bpf.Slot) error
	Stale() ([]bpf.Slot, error)
	SetIgnored([]netip.Prefix) error
	Read() (map[bpf.Slot]uint64, error)
}

// Source implements sablier.ActivitySource on top of a Tracker.
type Source struct {
	tracker     Tracker
	host        sablier.ActivityHost
	ignoreCIDRs []netip.Prefix
	log         *slog.Logger

	// ownAddrs lists the addresses of this host's own interfaces, loopback excluded.
	ownAddrs func() ([]netip.Addr, error)

	mu    sync.Mutex
	slots map[string]bpf.Slot
	last  map[bpf.Slot]uint64

	// ignoreWritten is set once an ignore list was written.
	ignoreWritten bool
	// ignoreComplete is set once a full ignore list (every lookup succeeded)
	// was written; after that a failed lookup keeps the previous list.
	ignoreComplete bool
}

var _ sablier.ActivitySource = (*Source)(nil)

// loopback is always ignored: container HEALTHCHECKs run inside the container
// and reach the service over loopback.
var loopback = []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8"), netip.MustParsePrefix("::1/128")}

func New(t Tracker, host sablier.ActivityHost, ignoreCIDRs []netip.Prefix, l *slog.Logger) *Source {
	if l == nil {
		l = slog.Default()
	}
	return &Source{
		tracker:     t,
		host:        host,
		ignoreCIDRs: ignoreCIDRs,
		log:         l,
		ownAddrs:    interfaceAddrs,
		slots:       map[string]bpf.Slot{},
		last:        map[bpf.Slot]uint64{},
	}
}

func interfaceAddrs() ([]netip.Addr, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}
	var out []netip.Addr
	for _, a := range addrs {
		n, ok := a.(*net.IPNet)
		if !ok {
			continue
		}
		ip, ok := netip.AddrFromSlice(n.IP)
		if !ok {
			continue
		}
		ip = ip.Unmap()
		if ip.IsLoopback() {
			continue
		}
		out = append(out, ip)
	}
	return out, nil
}

// toBPFPorts converts the ports to watch, skipping the ones that are neither
// TCP nor UDP, such as an exposed SCTP port: the eBPF programs only parse TCP
// and UDP. sablier.ActivityConfig ports are always TCP or UDP.
func (s *Source) toBPFPorts(ctx context.Context, instance string, ports []sablier.Port) []bpf.Port {
	out := make([]bpf.Port, 0, len(ports))
	for _, p := range ports {
		var proto uint8
		switch p.Proto {
		case "tcp":
			proto = bpf.ProtoTCP
		case "udp":
			proto = bpf.ProtoUDP
		default:
			s.log.DebugContext(ctx, "not watching a port that is neither TCP nor UDP",
				slog.String("instance", instance), slog.Any("port", p))
			continue
		}
		out = append(out, bpf.Port{Number: p.Number, Proto: proto})
	}
	return out
}

// Watch starts counting the instance's traffic. Watching an instance again
// replaces its previous slot.
func (s *Source) Watch(ctx context.Context, instance string, ports []sablier.Port) error {
	target, err := s.host.InstanceActivityTarget(ctx, instance)
	if err != nil {
		return err
	}
	if len(ports) == 0 {
		ports = target.Ports
	}
	bpfPorts := s.toBPFPorts(ctx, instance, ports)
	if len(bpfPorts) == 0 {
		return fmt.Errorf("instance %q: %w", instance, ErrNoPorts)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	// The kernel counts from the attach on: write the ignore list before the
	// first one, or the ignored sources count until the first Drain.
	if !s.ignoreWritten {
		if err := s.refreshIgnored(ctx); err != nil {
			s.log.WarnContext(ctx, "cannot refresh the activity ignore list", slog.Any("error", err))
		}
	}
	if err := s.unwatchLocked(instance); err != nil {
		return err
	}
	slot, err := s.tracker.Watch(target.CgroupPath, bpfPorts)
	if err != nil {
		return err
	}
	s.slots[instance] = slot
	s.last[slot] = 0
	return nil
}

// Unwatch stops counting the instance's traffic. Unknown instances are ignored.
func (s *Source) Unwatch(_ context.Context, instance string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.unwatchLocked(instance)
}

func (s *Source) unwatchLocked(instance string) error {
	slot, ok := s.slots[instance]
	if !ok {
		return nil
	}
	delete(s.slots, instance)
	delete(s.last, slot)
	return s.tracker.Unwatch(slot)
}

// Stale returns the watched instances whose cgroup is gone or was replaced
// since they were watched, as a container restart does, in order.
func (s *Source) Stale(context.Context) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	slots, err := s.tracker.Stale()
	var stale []string
	for instance, slot := range s.slots {
		if slices.Contains(slots, slot) {
			stale = append(stale, instance)
		}
	}
	slices.Sort(stale)
	return stale, err
}

// Drain returns the bytes seen per instance since the previous call, omitting
// instances without traffic.
func (s *Source) Drain(ctx context.Context) (map[string]uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.refreshIgnored(ctx); err != nil {
		s.log.WarnContext(ctx, "cannot refresh the activity ignore list", slog.Any("error", err))
	}
	totals, err := s.tracker.Read()
	if err != nil {
		return nil, err
	}
	out := map[string]uint64{}
	for instance, slot := range s.slots {
		total, ok := totals[slot]
		if !ok {
			continue
		}
		last := s.last[slot]
		s.last[slot] = total
		delta := total
		if total >= last {
			delta = total - last
		}
		if delta > 0 {
			out[instance] = delta
		}
	}
	return out, nil
}

func (s *Source) refreshIgnored(ctx context.Context) error {
	prefixes := append([]netip.Prefix{}, loopback...)
	prefixes = append(prefixes, s.ignoreCIDRs...)
	var errs []error
	var lookupFailed bool

	own, err := s.ownAddrs()
	if err != nil {
		errs = append(errs, err)
		lookupFailed = true
	}
	hostAddrs, err := s.host.ActivityIgnoredSources(ctx)
	if err != nil {
		errs = append(errs, err)
		lookupFailed = true
	}
	if lookupFailed && s.ignoreComplete {
		// Keep the last complete list rather than dropping ignored addresses.
		return errors.Join(errs...)
	}
	for _, a := range append(own, hostAddrs...) {
		prefixes = append(prefixes, netip.PrefixFrom(a, a.BitLen()))
	}
	if err := s.tracker.SetIgnored(prefixes); err != nil {
		errs = append(errs, err)
	} else {
		s.ignoreWritten = true
		s.ignoreComplete = s.ignoreComplete || !lookupFailed
	}
	return errors.Join(errs...)
}
