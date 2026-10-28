//go:build linux && (amd64 || arm64)

package bpf

import (
	"errors"
	"fmt"
	"net/netip"
	"os"
	"slices"
	"sync"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/rlimit"
)

// privilegesHint names what loading and attaching the programs needs.
const privilegesHint = "needs CAP_BPF and CAP_NET_ADMIN, and cgroup: host"

// Tracker counts the bytes each watched cgroup exchanges on its watched ports.
// It is safe for concurrent use.
// Nothing is pinned: the kernel frees the programs and maps once the Tracker
// is closed or the process exits.
type Tracker struct {
	mu      sync.Mutex
	spec    *ebpf.CollectionSpec
	maps    counterMaps
	slots   slotPool
	watches map[Slot]*watch
	ignored map[counterLpmKey]struct{}
}

// watch is one watched cgroup: its ports and its attached program pair.
type watch struct {
	ports           []Port
	ingress, egress *ebpf.Program
	links           []link.Link
}

// New loads the shared maps and checks that the programs load. It fails when
// cgroup v2 is missing or the process lacks the privileges.
func New() (*Tracker, error) {
	if _, err := os.Stat("/sys/fs/cgroup/cgroup.controllers"); err != nil {
		return nil, errors.New("cgroup v2 required")
	}
	if err := rlimit.RemoveMemlock(); err != nil {
		return nil, fmt.Errorf("remove the memlock limit (%s; before Linux 5.11 also CAP_SYS_RESOURCE): %w", privilegesHint, err)
	}
	spec, err := loadCounter()
	if err != nil {
		return nil, err
	}

	t := &Tracker{
		spec:    spec,
		watches: make(map[Slot]*watch),
		ignored: make(map[counterLpmKey]struct{}),
	}
	if err := spec.LoadAndAssign(&t.maps, nil); err != nil {
		return nil, fmt.Errorf("load eBPF maps (%s): %w", privilegesHint, err)
	}
	ingress, egress, err := t.loadPrograms(0)
	if err != nil {
		_ = t.maps.Close()
		return nil, fmt.Errorf("load eBPF programs (%s): %w", privilegesHint, err)
	}
	_ = ingress.Close()
	_ = egress.Close()
	return t, nil
}

// loadPrograms loads a program pair that counts into slot s and shares the
// Tracker's maps.
func (t *Tracker) loadPrograms(s Slot) (ingress, egress *ebpf.Program, err error) {
	spec := t.spec.Copy()
	if err := spec.Variables[counterVarSlot].Set(uint32(s)); err != nil {
		return nil, nil, fmt.Errorf("set slot %d: %w", s, err)
	}
	var progs counterPrograms
	err = spec.LoadAndAssign(&progs, &ebpf.CollectionOptions{
		MapReplacements: map[string]*ebpf.Map{
			counterMapPorts:   t.maps.Ports,
			counterMapIgnored: t.maps.Ignored,
			counterMapBytes:   t.maps.Bytes,
		},
	})
	if err != nil {
		return nil, nil, err
	}
	return progs.CountIngress, progs.CountEgress, nil
}

// setPorts adds the ports of slot s to the ports map.
func (t *Tracker) setPorts(s Slot, ports []Port) error {
	for _, p := range ports {
		if err := t.maps.Ports.Put(portKey(s, p), uint8(1)); err != nil {
			return fmt.Errorf("watch port %d/%d: %w", p.Number, p.Proto, err)
		}
	}
	return nil
}

// deletePorts removes the ports of slot s from the ports map.
func (t *Tracker) deletePorts(s Slot, ports []Port) error {
	var errs []error
	for _, p := range ports {
		err := t.maps.Ports.Delete(portKey(s, p))
		if err != nil && !errors.Is(err, ebpf.ErrKeyNotExist) {
			errs = append(errs, fmt.Errorf("unwatch port %d/%d: %w", p.Number, p.Proto, err))
		}
	}
	return errors.Join(errs...)
}

func portKey(s Slot, p Port) counterPortKey {
	return counterPortKey{Slot: uint32(s), Port: p.Number, Proto: p.Proto}
}

// Watch starts counting the traffic of the cgroup at cgroupPath (a cgroup v2
// directory) on ports. Its total starts at zero.
func (t *Tracker) Watch(cgroupPath string, ports []Port) (Slot, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	s, err := t.slots.alloc()
	if err != nil {
		return 0, err
	}
	w, err := t.watch(s, cgroupPath, ports)
	if err != nil {
		t.slots.free(s)
		return 0, err
	}
	t.watches[s] = w
	return s, nil
}

func (t *Tracker) watch(s Slot, cgroupPath string, ports []Port) (_ *watch, err error) {
	cpus, err := ebpf.PossibleCPU()
	if err != nil {
		return nil, err
	}
	if err := t.maps.Bytes.Put(uint32(s), make([]uint64, cpus)); err != nil {
		return nil, fmt.Errorf("reset the total of slot %d: %w", s, err)
	}

	w := &watch{ports: slices.Clone(ports)}
	defer func() {
		if err != nil {
			err = errors.Join(err, w.close(), t.deletePorts(s, ports))
		}
	}()
	if err := t.setPorts(s, ports); err != nil {
		return nil, err
	}
	if w.ingress, w.egress, err = t.loadPrograms(s); err != nil {
		return nil, fmt.Errorf("load eBPF programs: %w", err)
	}
	for _, a := range []struct {
		attach ebpf.AttachType
		prog   *ebpf.Program
	}{
		{ebpf.AttachCGroupInetIngress, w.ingress},
		{ebpf.AttachCGroupInetEgress, w.egress},
	} {
		l, err := link.AttachCgroup(link.CgroupOptions{Path: cgroupPath, Attach: a.attach, Program: a.prog})
		if err != nil {
			return nil, fmt.Errorf("attach to cgroup %s: %w", cgroupPath, err)
		}
		w.links = append(w.links, l)
	}
	return w, nil
}

// close detaches and unloads the program pair.
func (w *watch) close() error {
	var errs []error
	for _, l := range w.links {
		errs = append(errs, l.Close())
	}
	for _, p := range []*ebpf.Program{w.ingress, w.egress} {
		if p != nil {
			errs = append(errs, p.Close())
		}
	}
	return errors.Join(errs...)
}

// Unwatch stops counting slot s and frees it.
func (t *Tracker) Unwatch(s Slot) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	w, ok := t.watches[s]
	if !ok {
		return fmt.Errorf("activity tracking: slot %d is not watched", s)
	}
	delete(t.watches, s)
	err := errors.Join(w.close(), t.deletePorts(s, w.ports))
	t.slots.free(s)
	return err
}

// SetIgnored replaces the remote address prefixes whose traffic isn't counted.
func (t *Tracker) SetIgnored(prefixes []netip.Prefix) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	want := make(map[counterLpmKey]struct{}, len(prefixes))
	for _, p := range prefixes {
		if !p.IsValid() {
			return fmt.Errorf("ignore %v: invalid prefix", p)
		}
		want[lpmKey(p)] = struct{}{}
	}
	// Add before removing, so an address both sets cover stays ignored throughout.
	for k := range want {
		if err := t.maps.Ignored.Put(k, uint8(1)); err != nil {
			return fmt.Errorf("ignore prefix: %w", err)
		}
		t.ignored[k] = struct{}{}
	}
	for k := range t.ignored {
		if _, ok := want[k]; ok {
			continue
		}
		if err := t.maps.Ignored.Delete(k); err != nil && !errors.Is(err, ebpf.ErrKeyNotExist) {
			return fmt.Errorf("stop ignoring prefix: %w", err)
		}
		delete(t.ignored, k)
	}
	return nil
}

// lpmKey stores IPv4 prefixes as IPv4-mapped IPv6, as counter.c looks them up.
func lpmKey(p netip.Prefix) counterLpmKey {
	p = p.Masked()
	bits := p.Bits()
	if p.Addr().Is4() {
		bits += 96
	}
	return counterLpmKey{Prefixlen: uint32(bits), Addr: p.Addr().As16()}
}

// Read returns the running byte total of every watched slot.
func (t *Tracker) Read() (map[Slot]uint64, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	totals := make(map[Slot]uint64)
	var perCPU []uint64
	for i, used := range t.slots.used {
		if !used {
			continue
		}
		if err := t.maps.Bytes.Lookup(uint32(i), &perCPU); err != nil {
			return nil, fmt.Errorf("read the total of slot %d: %w", i, err)
		}
		var sum uint64
		for _, n := range perCPU {
			sum += n
		}
		totals[Slot(i)] = sum
	}
	return totals, nil
}

// Close stops every watch and frees the maps.
func (t *Tracker) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	var errs []error
	for s, w := range t.watches {
		errs = append(errs, w.close())
		delete(t.watches, s)
		t.slots.free(s)
	}
	errs = append(errs, t.maps.Close())
	return errors.Join(errs...)
}
