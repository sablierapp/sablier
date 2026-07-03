package sablier

// Internal tests for anti-affinity. They live in package sablier (not
// sablier_test) so they can drive the unexported reconcileAntiAffinity directly
// and inspect internal state. Importing providertest/inmemory here would create
// an import cycle, so a minimal fake store and provider are used instead.

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/store"
	"gotest.tools/v3/assert"
)

// --- fakes ---

type fakeAAStore struct {
	mu   sync.Mutex
	data map[string]InstanceInfo
}

func newFakeAAStore() *fakeAAStore { return &fakeAAStore{data: map[string]InstanceInfo{}} }

func (f *fakeAAStore) Get(_ context.Context, k string) (InstanceInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.data[k]
	if !ok {
		return InstanceInfo{}, store.ErrKeyNotFound
	}
	return v, nil
}

func (f *fakeAAStore) Put(_ context.Context, v InstanceInfo, _ time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.data[v.Name] = v
	return nil
}

func (f *fakeAAStore) Delete(_ context.Context, k string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.data, k)
	return nil
}

func (f *fakeAAStore) OnExpire(context.Context, func(string)) error { return nil }

func (f *fakeAAStore) Range(_ context.Context, fn func(InstanceInfo, time.Time)) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, v := range f.data {
		fn(v, time.Time{})
	}
	return nil
}

func (f *fakeAAStore) session(name string) {
	_ = f.Put(context.Background(), InstanceInfo{Name: name, Status: InstanceStatusReady}, time.Minute)
}

type fakeAAProvider struct {
	mu      sync.Mutex
	started []string
	stopped []string
}

func (f *fakeAAProvider) InstanceStart(_ context.Context, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.started = append(f.started, name)
	return nil
}

func (f *fakeAAProvider) InstanceStop(_ context.Context, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopped = append(f.stopped, name)
	return nil
}

func (f *fakeAAProvider) InstanceInspect(_ context.Context, name string) (InstanceInfo, error) {
	return InstanceInfo{Name: name}, nil
}

func (f *fakeAAProvider) InstanceGroups(context.Context) (map[string][]string, error) {
	return nil, nil
}

func (f *fakeAAProvider) InstanceList(context.Context, provider.InstanceListOptions) ([]InstanceConfiguration, error) {
	return nil, nil
}

func (f *fakeAAProvider) InstanceDependencies(context.Context, string) ([]InstanceDependency, error) {
	return nil, nil
}

func (f *fakeAAProvider) InstanceEvents(context.Context, provider.InstanceEventsOptions) InstanceEventStream {
	return InstanceEventStream{}
}

func (f *fakeAAProvider) snapshotStopped() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.stopped)
}

func (f *fakeAAProvider) snapshotStarted() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.started)
}

func setupAntiAffinity(t *testing.T) (*Sablier, *fakeAAStore, *fakeAAProvider) {
	t.Helper()
	st := newFakeAAStore()
	p := &fakeAAProvider{}
	s := New(slogt.New(t), st, p)
	return s, st, p
}

func (s *Sablier) isSuppressed(name string) bool {
	s.affinityMu.Lock()
	defer s.affinityMu.Unlock()
	_, ok := s.suppressed[name]
	return ok
}

// --- tests ---

func TestReconcileAntiAffinity_SuppressesActiveDependent(t *testing.T) {
	s, st, p := setupAntiAffinity(t)
	ctx := context.Background()

	s.SetGroups(map[string][]string{"streaming": {"plex"}})
	s.SyncInstanceAntiAffinity("nextcloud", []string{"streaming"})

	st.session("plex")      // streaming group is active
	st.session("nextcloud") // dependent is currently running

	s.reconcileAntiAffinity(ctx)

	assert.Assert(t, slices.Contains(p.snapshotStopped(), "nextcloud"), "dependent should be forced idle")
	assert.Assert(t, s.isSuppressed("nextcloud"), "dependent should be recorded as suppressed")
	_, err := st.Get(ctx, "nextcloud")
	assert.ErrorIs(t, err, store.ErrKeyNotFound, "suppressed dependent's session should be cleared")
}

func TestReconcileAntiAffinity_RestoresWhenGroupInactive(t *testing.T) {
	s, st, p := setupAntiAffinity(t)
	ctx := context.Background()

	s.SetGroups(map[string][]string{"streaming": {"plex"}})
	s.SyncInstanceAntiAffinity("nextcloud", []string{"streaming"})

	// Activate then suppress.
	st.session("plex")
	st.session("nextcloud")
	s.reconcileAntiAffinity(ctx)
	assert.Assert(t, s.isSuppressed("nextcloud"))

	// Antagonist session ends -> group inactive -> dependent must be restored.
	_ = st.Delete(ctx, "plex")
	s.reconcileAntiAffinity(ctx)

	assert.Assert(t, slices.Contains(p.snapshotStarted(), "nextcloud"), "dependent should be restored")
	assert.Assert(t, !s.isSuppressed("nextcloud"), "dependent should no longer be suppressed")
}

func TestReconcileAntiAffinity_DoesNotSuppressAlreadyIdleDependent(t *testing.T) {
	s, st, p := setupAntiAffinity(t)
	ctx := context.Background()

	s.SetGroups(map[string][]string{"streaming": {"plex"}})
	s.SyncInstanceAntiAffinity("nextcloud", []string{"streaming"})

	st.session("plex") // active, but nextcloud has no session (already idle)

	s.reconcileAntiAffinity(ctx)

	assert.Assert(t, !slices.Contains(p.snapshotStopped(), "nextcloud"), "already-idle dependent must not be stopped")
	assert.Assert(t, !s.isSuppressed("nextcloud"), "already-idle dependent must not be recorded as suppressed")
}

func TestReconcileAntiAffinity_RestoreOnlyWhenAllAntagonistsInactive(t *testing.T) {
	s, st, p := setupAntiAffinity(t)
	ctx := context.Background()

	s.SetGroups(map[string][]string{"streaming": {"plex"}, "backup": {"restic"}})
	s.SyncInstanceAntiAffinity("nextcloud", []string{"streaming", "backup"})

	st.session("nextcloud")
	st.session("restic") // only backup is active
	s.reconcileAntiAffinity(ctx)
	assert.Assert(t, s.isSuppressed("nextcloud"), "dependent should be suppressed while any antagonist is active")

	// streaming becomes active too; backup ends. Still one antagonist active.
	_ = st.Delete(ctx, "restic")
	st.session("plex")
	s.reconcileAntiAffinity(ctx)
	assert.Assert(t, s.isSuppressed("nextcloud"), "dependent should stay suppressed while streaming is active")
	assert.Assert(t, !slices.Contains(p.snapshotStarted(), "nextcloud"), "must not restore while an antagonist is active")

	// All antagonists inactive -> restore.
	_ = st.Delete(ctx, "plex")
	s.reconcileAntiAffinity(ctx)
	assert.Assert(t, slices.Contains(p.snapshotStarted(), "nextcloud"), "dependent should be restored once all antagonists are inactive")
	assert.Assert(t, !s.isSuppressed("nextcloud"))
}

func TestReconcileAntiAffinity_NoOpWhenNoAntiAffinity(t *testing.T) {
	s, st, p := setupAntiAffinity(t)
	ctx := context.Background()

	s.SetGroups(map[string][]string{"streaming": {"plex"}})
	st.session("plex")

	s.reconcileAntiAffinity(ctx)

	assert.Equal(t, len(p.snapshotStopped()), 0)
	assert.Equal(t, len(p.snapshotStarted()), 0)
}

func TestParseAntiAffinity(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{name: "single group", input: "streaming", want: []string{"streaming"}},
		{name: "multiple groups", input: "streaming,backup", want: []string{"streaming", "backup"}},
		{name: "trims spaces", input: " a , b ", want: []string{"a", "b"}},
		{name: "deduplicates", input: "a,a,b", want: []string{"a", "b"}},
		{name: "empty", input: "", want: nil},
		{name: "only commas and spaces", input: " , , ", want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.DeepEqual(t, ParseAntiAffinity(tt.input), tt.want)
		})
	}
}
