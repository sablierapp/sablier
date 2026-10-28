//go:build linux && (amd64 || arm64)

package bpf

import (
	"encoding/binary"
	"errors"
	"io/fs"
	"net/netip"
	"os"
	"slices"
	"testing"

	"github.com/cilium/ebpf"
	"gotest.tools/v3/assert"
)

const ethHeaderLen = 14

var (
	local   = netip.MustParseAddr("10.0.0.2")
	remote  = netip.MustParseAddr("10.0.0.9")
	local6  = netip.MustParseAddr("fd00::2")
	remote6 = netip.MustParseAddr("fd00::9")
)

func needsRoot(t *testing.T) {
	t.Helper()
	if os.Geteuid() != 0 {
		t.Skip("needs root: go test -exec sudo")
	}
}

// frame builds an Ethernet frame holding an IPv4 or IPv6 packet (the family
// of src) with a TCP or UDP header and payload zero bytes. Checksums stay
// zero: nothing on the test path checks them.
func frame(src, dst netip.Addr, proto uint8, sport, dport uint16, payload int) []byte {
	var l4 []byte
	switch proto {
	case ProtoTCP:
		l4 = make([]byte, 20+payload)
		l4[12] = 5 << 4 // data offset: 5 words
		l4[13] = 0x18   // PSH, ACK
		binary.BigEndian.PutUint16(l4[14:], 65535)
	case ProtoUDP:
		l4 = make([]byte, 8+payload)
		binary.BigEndian.PutUint16(l4[4:], uint16(len(l4)))
	default:
		panic("frame: protocol must be TCP or UDP")
	}
	binary.BigEndian.PutUint16(l4[0:], sport)
	binary.BigEndian.PutUint16(l4[2:], dport)

	eth := make([]byte, ethHeaderLen)
	var ip []byte
	if src.Is4() {
		binary.BigEndian.PutUint16(eth[12:], 0x0800)
		ip = make([]byte, 20)
		ip[0] = 4<<4 | 5 // version, header length in words
		binary.BigEndian.PutUint16(ip[2:], uint16(len(ip)+len(l4)))
		ip[8] = 64 // TTL
		ip[9] = proto
		s, d := src.As4(), dst.As4()
		copy(ip[12:], s[:])
		copy(ip[16:], d[:])
	} else {
		binary.BigEndian.PutUint16(eth[12:], 0x86dd)
		ip = make([]byte, 40)
		ip[0] = 6 << 4 // version
		binary.BigEndian.PutUint16(ip[4:], uint16(len(l4)))
		ip[6] = proto // next header
		ip[7] = 64    // hop limit
		s, d := src.As16(), dst.As16()
		copy(ip[8:], s[:])
		copy(ip[24:], d[:])
	}
	return slices.Concat(eth, ip, l4)
}

// withIPv6HopByHop marks an IPv6 frame as followed by a hop-by-hop options
// header. The TCP or UDP header stays right after the IPv6 header, so the
// frame would count if the program parsed past the extension header.
func withIPv6HopByHop(f []byte) []byte {
	f[ethHeaderLen+6] = 0
	return f
}

// withIPv4FragmentOffset makes an IPv4 frame a later fragment (offset 8 bytes).
func withIPv4FragmentOffset(f []byte) []byte {
	binary.BigEndian.PutUint16(f[ethHeaderLen+6:], 1)
	return f
}

// counter is a slot of a fresh Tracker with its programs loaded but not
// attached, so frames reach them through prog.Run instead of a cgroup.
type counter struct {
	tr              *Tracker
	slot            Slot
	ingress, egress *ebpf.Program
}

func newCounter(t *testing.T, ports ...Port) counter {
	t.Helper()
	tr, err := New()
	assert.NilError(t, err)
	t.Cleanup(func() { assert.NilError(t, tr.Close()) })

	// Keep slot 0 busy so a program that ignores its slot constant counts
	// into the wrong slot and fails the test.
	_, err = tr.slots.alloc()
	assert.NilError(t, err)
	s, err := tr.slots.alloc()
	assert.NilError(t, err)

	assert.NilError(t, tr.setPorts(s, ports))
	ingress, egress, err := tr.loadPrograms(s)
	assert.NilError(t, err)
	t.Cleanup(func() {
		assert.NilError(t, ingress.Close())
		assert.NilError(t, egress.Close())
	})
	return counter{tr: tr, slot: s, ingress: ingress, egress: egress}
}

func (c counter) total(t *testing.T) uint64 {
	t.Helper()
	totals, err := c.tr.Read()
	assert.NilError(t, err)
	return totals[c.slot]
}

func run(t *testing.T, prog *ebpf.Program, frame []byte) uint32 {
	t.Helper()
	ret, err := prog.Run(&ebpf.RunOptions{Data: frame})
	assert.NilError(t, err)
	return ret
}

func TestCounter_CountsWatchedPortBothDirections(t *testing.T) {
	needsRoot(t)
	c := newCounter(t, Port{Number: 8080, Proto: ProtoTCP})

	in := frame(remote, local, ProtoTCP, 51000, 8080, 100)
	out := frame(local, remote, ProtoTCP, 8080, 51000, 1000)
	run(t, c.ingress, in)
	run(t, c.egress, out)
	// The other way round the local port is 51000, which isn't watched.
	run(t, c.egress, in)
	run(t, c.ingress, out)

	// Under cgroup_skb, skb->len starts at the IP header.
	want := uint64(len(in) - ethHeaderLen + len(out) - ethHeaderLen)
	totals, err := c.tr.Read()
	assert.NilError(t, err)
	assert.DeepEqual(t, totals, map[Slot]uint64{0: 0, c.slot: want})
}

func TestCounter_SkipsUnwatchedPort(t *testing.T) {
	needsRoot(t)
	c := newCounter(t, Port{Number: 8080, Proto: ProtoTCP})

	run(t, c.ingress, frame(remote, local, ProtoTCP, 51000, 9090, 100))
	run(t, c.ingress, frame(remote, local, ProtoUDP, 51000, 8080, 100))

	assert.Equal(t, c.total(t), uint64(0))
}

func TestCounter_SkipsIgnoredSource(t *testing.T) {
	needsRoot(t)
	c := newCounter(t, Port{Number: 8080, Proto: ProtoTCP})
	assert.NilError(t, c.tr.SetIgnored([]netip.Prefix{
		netip.MustParsePrefix("10.0.0.9/32"),
		netip.MustParsePrefix("fd00::9/128"),
	}))

	run(t, c.ingress, frame(remote, local, ProtoTCP, 51000, 8080, 100))
	run(t, c.ingress, frame(remote6, local6, ProtoTCP, 51000, 8080, 100))
	// On egress the remote address is the destination.
	run(t, c.egress, frame(local, remote, ProtoTCP, 8080, 51000, 100))
	assert.Equal(t, c.total(t), uint64(0))

	in4 := frame(netip.MustParseAddr("10.0.0.10"), local, ProtoTCP, 51000, 8080, 100)
	in6 := frame(netip.MustParseAddr("fd00::10"), local6, ProtoTCP, 51000, 8080, 200)
	run(t, c.ingress, in4)
	run(t, c.ingress, in6)
	assert.Equal(t, c.total(t), uint64(len(in4)-ethHeaderLen+len(in6)-ethHeaderLen))
}

func TestCounter_IPv6UDP(t *testing.T) {
	needsRoot(t)
	c := newCounter(t, Port{Number: 5353, Proto: ProtoUDP})

	in := frame(remote6, local6, ProtoUDP, 5353, 5353, 64)
	run(t, c.ingress, in)

	assert.Equal(t, c.total(t), uint64(len(in)-ethHeaderLen))
}

func TestCounter_SkipsIPv6ExtensionHeader(t *testing.T) {
	needsRoot(t)
	c := newCounter(t, Port{Number: 8080, Proto: ProtoTCP})

	plain := frame(remote6, local6, ProtoTCP, 51000, 8080, 100)
	run(t, c.ingress, plain)
	assert.Equal(t, c.total(t), uint64(len(plain)-ethHeaderLen))

	run(t, c.ingress, withIPv6HopByHop(frame(remote6, local6, ProtoTCP, 51000, 8080, 100)))
	assert.Equal(t, c.total(t), uint64(len(plain)-ethHeaderLen))
}

func TestCounter_SkipsIPv4LaterFragment(t *testing.T) {
	needsRoot(t)
	c := newCounter(t, Port{Number: 8080, Proto: ProtoTCP})

	run(t, c.ingress, withIPv4FragmentOffset(frame(remote, local, ProtoTCP, 51000, 8080, 100)))

	assert.Equal(t, c.total(t), uint64(0))
}

func TestCounter_AlwaysAllows(t *testing.T) {
	needsRoot(t)
	c := newCounter(t, Port{Number: 8080, Proto: ProtoTCP}, Port{Number: 5353, Proto: ProtoUDP})
	assert.NilError(t, c.tr.SetIgnored([]netip.Prefix{
		netip.MustParsePrefix("10.0.0.9/32"),
		netip.MustParsePrefix("fd00::9/128"),
	}))

	frames := map[string][]byte{
		"watched in":      frame(netip.MustParseAddr("10.0.0.10"), local, ProtoTCP, 51000, 8080, 100),
		"watched out":     frame(local, netip.MustParseAddr("10.0.0.10"), ProtoTCP, 8080, 51000, 1000),
		"unwatched":       frame(remote, local, ProtoTCP, 51000, 9090, 100),
		"ignored v4":      frame(remote, local, ProtoTCP, 51000, 8080, 100),
		"ignored v6":      frame(remote6, local6, ProtoTCP, 51000, 8080, 100),
		"ipv6 udp":        frame(remote6, local6, ProtoUDP, 5353, 5353, 64),
		"ipv6 hop-by-hop": withIPv6HopByHop(frame(remote6, local6, ProtoTCP, 51000, 8080, 100)),
		"ipv4 fragment":   withIPv4FragmentOffset(frame(remote, local, ProtoTCP, 51000, 8080, 100)),
	}
	for name, f := range frames {
		assert.Equal(t, run(t, c.ingress, f), uint32(1), "ingress: %s", name)
		assert.Equal(t, run(t, c.egress, f), uint32(1), "egress: %s", name)
	}
}

func TestTracker_UnwatchThenWatchStartsAtZero(t *testing.T) {
	needsRoot(t)
	cgroup, err := os.MkdirTemp("/sys/fs/cgroup", "sablier-test-")
	assert.NilError(t, err)
	t.Cleanup(func() { assert.NilError(t, os.Remove(cgroup)) })
	tr, err := New()
	assert.NilError(t, err)
	t.Cleanup(func() { assert.NilError(t, tr.Close()) })
	ports := []Port{{Number: 8080, Proto: ProtoTCP}}

	s, err := tr.Watch(cgroup, ports)
	assert.NilError(t, err)
	in := frame(remote, local, ProtoTCP, 51000, 8080, 100)
	run(t, tr.watches[s].ingress, in)
	totals, err := tr.Read()
	assert.NilError(t, err)
	assert.DeepEqual(t, totals, map[Slot]uint64{s: uint64(len(in) - ethHeaderLen)})

	assert.NilError(t, tr.Unwatch(s))
	totals, err = tr.Read()
	assert.NilError(t, err)
	assert.DeepEqual(t, totals, map[Slot]uint64{})

	again, err := tr.Watch(cgroup, ports)
	assert.NilError(t, err)
	assert.Equal(t, again, s)
	totals, err = tr.Read()
	assert.NilError(t, err)
	assert.DeepEqual(t, totals, map[Slot]uint64{s: 0})
}

// A restarted container gets a new cgroup at the same path; the programs stay
// on the old one. cgroupfs never reuses a cgroup's inode, so removing and
// creating the directory again is a restart.
func TestTracker_StaleWhenCgroupRecreated(t *testing.T) {
	needsRoot(t)
	cgroup, err := os.MkdirTemp("/sys/fs/cgroup", "sablier-test-")
	assert.NilError(t, err)
	t.Cleanup(func() { _ = os.Remove(cgroup) })
	tr, err := New()
	assert.NilError(t, err)
	t.Cleanup(func() { assert.NilError(t, tr.Close()) })
	ports := []Port{{Number: 8080, Proto: ProtoTCP}}
	s, err := tr.Watch(cgroup, ports)
	assert.NilError(t, err)

	stale, err := tr.Stale()
	assert.NilError(t, err)
	assert.Equal(t, len(stale), 0)

	assert.NilError(t, os.Remove(cgroup))
	stale, err = tr.Stale()
	assert.NilError(t, err)
	assert.DeepEqual(t, stale, []Slot{s})

	assert.NilError(t, os.Mkdir(cgroup, 0o755))
	stale, err = tr.Stale()
	assert.NilError(t, err)
	assert.DeepEqual(t, stale, []Slot{s})

	// Watching the new cgroup is current again.
	assert.NilError(t, tr.Unwatch(s))
	_, err = tr.Watch(cgroup, ports)
	assert.NilError(t, err)
	stale, err = tr.Stale()
	assert.NilError(t, err)
	assert.Equal(t, len(stale), 0)
}

// Without cgroup: host, Sablier sees its own cgroup namespace and the
// containers' cgroups are missing. Needs no privileges: Watch fails before it
// touches the kernel.
func TestTracker_WatchMissingCgroup(t *testing.T) {
	tr := &Tracker{watches: make(map[Slot]*watch), ignored: make(map[counterLpmKey]struct{})}
	path := "/sys/fs/cgroup/sablier-test-missing/0123abcd"

	_, err := tr.Watch(path, []Port{{Number: 8080, Proto: ProtoTCP}})

	assert.Error(t, err, "cgroup "+path+" not found: run Sablier with cgroup: host on the Docker host")
	assert.Assert(t, errors.Is(err, fs.ErrNotExist), "%v", err)
	assert.Assert(t, !slices.Contains(tr.slots.used[:], true), "a slot leaked")
}
