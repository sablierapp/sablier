package local

import (
	"context"
	"errors"
	"net/netip"
	"slices"
	"testing"

	"go.uber.org/mock/gomock"
	"gotest.tools/v3/assert"

	"github.com/sablierapp/sablier/pkg/activity/bpf"
	"github.com/sablierapp/sablier/pkg/provider/providertest"
	"github.com/sablierapp/sablier/pkg/sablier"
)

type fakeTracker struct {
	used    map[bpf.Slot]bool
	totals  map[bpf.Slot]uint64
	calls   []string
	watched [][]bpf.Port
	ignored []netip.Prefix
}

func newFakeTracker() *fakeTracker {
	return &fakeTracker{used: map[bpf.Slot]bool{}, totals: map[bpf.Slot]uint64{}}
}

func (f *fakeTracker) Watch(_ string, ports []bpf.Port) (bpf.Slot, error) {
	for s := bpf.Slot(0); ; s++ {
		if !f.used[s] {
			f.used[s] = true
			f.totals[s] = 0
			f.calls = append(f.calls, "watch")
			f.watched = append(f.watched, ports)
			return s, nil
		}
	}
}

func (f *fakeTracker) Unwatch(s bpf.Slot) error {
	delete(f.used, s)
	delete(f.totals, s)
	f.calls = append(f.calls, "unwatch")
	return nil
}

func (f *fakeTracker) SetIgnored(p []netip.Prefix) error {
	f.ignored = p
	f.calls = append(f.calls, "setignored")
	return nil
}

func (f *fakeTracker) Read() (map[bpf.Slot]uint64, error) {
	f.calls = append(f.calls, "read")
	out := map[bpf.Slot]uint64{}
	for s := range f.used {
		out[s] = f.totals[s]
	}
	return out, nil
}

func newSource(t *testing.T, ft *fakeTracker, cidrs ...netip.Prefix) (*Source, *providertest.MockActivityHost) {
	t.Helper()
	host := providertest.NewMockActivityHost(gomock.NewController(t))
	s := New(ft, host, cidrs, nil)
	s.ownAddrs = func() ([]netip.Addr, error) { return nil, nil }
	return s, host
}

func expectTarget(host *providertest.MockActivityHost, name string, ports ...sablier.Port) {
	host.EXPECT().InstanceActivityTarget(gomock.Any(), name).
		Return(sablier.ActivityTarget{CgroupPath: "/cg/" + name, Ports: ports}, nil).AnyTimes()
}

func TestSource_WatchUsesLabelPortsOverExposed(t *testing.T) {
	ft := newFakeTracker()
	s, host := newSource(t, ft)
	expectTarget(host, "plex", sablier.Port{Number: 32400, Proto: "tcp"}, sablier.Port{Number: 1900, Proto: "udp"})
	err := s.Watch(context.Background(), "plex", []sablier.Port{{Number: 32400, Proto: "tcp"}})
	assert.NilError(t, err)
	assert.DeepEqual(t, ft.watched, [][]bpf.Port{{{Number: 32400, Proto: bpf.ProtoTCP}}})
}

func TestSource_WatchFallsBackToExposedPorts(t *testing.T) {
	ft := newFakeTracker()
	s, host := newSource(t, ft)
	expectTarget(host, "plex", sablier.Port{Number: 32400, Proto: "tcp"}, sablier.Port{Number: 1900, Proto: "udp"})
	assert.NilError(t, s.Watch(context.Background(), "plex", nil))
	assert.DeepEqual(t, ft.watched, [][]bpf.Port{{{Number: 32400, Proto: bpf.ProtoTCP}, {Number: 1900, Proto: bpf.ProtoUDP}}})
}

func TestSource_WatchWithoutPortsFails(t *testing.T) {
	ft := newFakeTracker()
	s, host := newSource(t, ft)
	expectTarget(host, "plex")
	err := s.Watch(context.Background(), "plex", nil)
	assert.Assert(t, errors.Is(err, ErrNoPorts))
	assert.Equal(t, len(ft.watched), 0)
}

func TestSource_WatchTwiceReplacesSlot(t *testing.T) {
	ft := newFakeTracker()
	s, host := newSource(t, ft)
	expectTarget(host, "plex", sablier.Port{Number: 1, Proto: "tcp"})
	assert.NilError(t, s.Watch(context.Background(), "plex", nil))
	assert.NilError(t, s.Watch(context.Background(), "plex", nil))
	assert.DeepEqual(t, ft.calls, []string{"watch", "unwatch", "watch"})
	assert.Equal(t, len(ft.used), 1)
}

func TestSource_DrainReturnsDeltas(t *testing.T) {
	ft := newFakeTracker()
	s, host := newSource(t, ft)
	expectTarget(host, "plex", sablier.Port{Number: 1, Proto: "tcp"})
	host.EXPECT().ActivityIgnoredSources(gomock.Any()).Return(nil, nil).AnyTimes()
	assert.NilError(t, s.Watch(context.Background(), "plex", nil))

	ft.totals[0] = 100
	got, err := s.Drain(context.Background())
	assert.NilError(t, err)
	assert.DeepEqual(t, got, map[string]uint64{"plex": 100})

	ft.totals[0] = 250
	got, err = s.Drain(context.Background())
	assert.NilError(t, err)
	assert.DeepEqual(t, got, map[string]uint64{"plex": 150})
}

func TestSource_DrainOmitsZeroDeltas(t *testing.T) {
	ft := newFakeTracker()
	s, host := newSource(t, ft)
	expectTarget(host, "plex", sablier.Port{Number: 1, Proto: "tcp"})
	host.EXPECT().ActivityIgnoredSources(gomock.Any()).Return(nil, nil).AnyTimes()
	assert.NilError(t, s.Watch(context.Background(), "plex", nil))

	ft.totals[0] = 100
	got, _ := s.Drain(context.Background())
	assert.DeepEqual(t, got, map[string]uint64{"plex": 100})
	got, err := s.Drain(context.Background())
	assert.NilError(t, err)
	assert.DeepEqual(t, got, map[string]uint64{})
}

func TestSource_DrainAfterSlotReuseDoesNotWrap(t *testing.T) {
	ft := newFakeTracker()
	s, host := newSource(t, ft)
	expectTarget(host, "a", sablier.Port{Number: 1, Proto: "tcp"})
	expectTarget(host, "b", sablier.Port{Number: 1, Proto: "tcp"})
	host.EXPECT().ActivityIgnoredSources(gomock.Any()).Return(nil, nil).AnyTimes()
	ctx := context.Background()

	assert.NilError(t, s.Watch(ctx, "a", nil))
	ft.totals[0] = 1000
	got, _ := s.Drain(ctx)
	assert.DeepEqual(t, got, map[string]uint64{"a": 1000})

	assert.NilError(t, s.Unwatch(ctx, "a"))
	assert.NilError(t, s.Watch(ctx, "b", nil))
	ft.totals[0] = 10
	got, err := s.Drain(ctx)
	assert.NilError(t, err)
	assert.DeepEqual(t, got, map[string]uint64{"b": 10})
}

func TestSource_DrainRefreshesIgnoreList(t *testing.T) {
	ft := newFakeTracker()
	s, host := newSource(t, ft, netip.MustParsePrefix("192.0.2.0/24"))
	s.ownAddrs = func() ([]netip.Addr, error) { return []netip.Addr{netip.MustParseAddr("172.18.0.5")}, nil }
	host.EXPECT().ActivityIgnoredSources(gomock.Any()).
		Return([]netip.Addr{netip.MustParseAddr("172.18.0.9"), netip.MustParseAddr("fd00::9")}, nil)

	_, err := s.Drain(context.Background())
	assert.NilError(t, err)

	want := []string{"172.18.0.5/32", "172.18.0.9/32", "fd00::9/128", "192.0.2.0/24", "127.0.0.0/8", "::1/128"}
	var got []string
	for _, p := range ft.ignored {
		got = append(got, p.String())
	}
	slices.Sort(got)
	slices.Sort(want)
	assert.DeepEqual(t, got, want)
	assert.DeepEqual(t, ft.calls, []string{"setignored", "read"})
}

func TestSource_DrainKeepsIgnoreListWhenLookupFails(t *testing.T) {
	ft := newFakeTracker()
	s, host := newSource(t, ft)
	expectTarget(host, "plex", sablier.Port{Number: 1, Proto: "tcp"})
	gomock.InOrder(
		host.EXPECT().ActivityIgnoredSources(gomock.Any()).Return([]netip.Addr{netip.MustParseAddr("172.18.0.9")}, nil),
		host.EXPECT().ActivityIgnoredSources(gomock.Any()).Return(nil, errors.New("docker unavailable")),
	)
	ctx := context.Background()
	assert.NilError(t, s.Watch(ctx, "plex", nil))

	_, err := s.Drain(ctx)
	assert.NilError(t, err)
	first := len(ft.ignored)
	assert.Assert(t, first > 0)
	ft.calls = nil

	ft.totals[0] = 7
	got, err := s.Drain(ctx)
	assert.NilError(t, err)
	assert.DeepEqual(t, got, map[string]uint64{"plex": 7})
	assert.DeepEqual(t, ft.calls, []string{"read"})
	assert.Equal(t, len(ft.ignored), first)
	assert.Assert(t, slices.Contains(ft.ignored, netip.MustParsePrefix("172.18.0.9/32")))
}
