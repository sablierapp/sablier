package sablier

// Internal tests for anti-affinity. They live in package sablier (not
// sablier_test) so they can drive the unexported reconcileAntiAffinity directly
// and inspect internal state. Importing providertest/inmemory here would create
// an import cycle, so a minimal fake store and provider are used instead.

import (
	"context"
	"errors"
	"slices"
	"strings"
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
	// getErr, when set, is returned by Get for every key (simulating a store
	// failure that is not ErrKeyNotFound). deleteErr/putErr behave likewise for
	// Delete/Put.
	getErr    error
	deleteErr error
	putErr    error
}

func newFakeAAStore() *fakeAAStore { return &fakeAAStore{data: map[string]InstanceInfo{}} }

func (f *fakeAAStore) Get(_ context.Context, k string) (InstanceInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.getErr != nil {
		return InstanceInfo{}, f.getErr
	}
	v, ok := f.data[k]
	if !ok {
		return InstanceInfo{}, store.ErrKeyNotFound
	}
	return v, nil
}

func (f *fakeAAStore) Put(_ context.Context, v InstanceInfo, _ time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.putErr != nil {
		return f.putErr
	}
	f.data[v.Name] = v
	return nil
}

func (f *fakeAAStore) Delete(_ context.Context, k string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.deleteErr != nil {
		return f.deleteErr
	}
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
	mu       sync.Mutex
	started  []string
	stopped  []string
	startErr error
	stopErr  error
	// list/listErr back InstanceList; inspect/inspectErr back InstanceInspect
	// (keyed by instance name). Unset inspect entries fall back to a bare
	// InstanceInfo{Name: name}.
	list       []InstanceConfiguration
	listErr    error
	inspect    map[string]InstanceInfo
	inspectErr map[string]error
}

func (f *fakeAAProvider) InstanceStart(_ context.Context, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.started = append(f.started, name)
	return f.startErr
}

func (f *fakeAAProvider) InstanceStop(_ context.Context, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopped = append(f.stopped, name)
	return f.stopErr
}

func (f *fakeAAProvider) InstanceInspect(_ context.Context, name string) (InstanceInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.inspectErr[name]; err != nil {
		return InstanceInfo{}, err
	}
	if info, ok := f.inspect[name]; ok {
		return info, nil
	}
	return InstanceInfo{Name: name}, nil
}

func (f *fakeAAProvider) InstanceGroups(context.Context) (map[string][]string, error) {
	return nil, nil
}

func (f *fakeAAProvider) InstanceList(context.Context, provider.InstanceListOptions) ([]InstanceConfiguration, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.list, f.listErr
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

func TestPopulateEnabledAndGroup_AntiAffinity(t *testing.T) {
	var info InstanceInfo
	PopulateEnabledAndGroup(&info, map[string]string{
		"sablier.enable":        "true",
		"sablier.anti-affinity": "streaming, backup",
		"sablier.group":         "myapp",
	})
	assert.DeepEqual(t, info.AntiAffinity, []string{"streaming", "backup"})

	// No label -> nil.
	var none InstanceInfo
	PopulateEnabledAndGroup(&none, map[string]string{"sablier.enable": "true"})
	assert.Assert(t, none.AntiAffinity == nil)
}

func TestReconcileAntiAffinityRegistry_BuildsAndPrunes(t *testing.T) {
	s, _, p := setupAntiAffinity(t)
	ctx := context.Background()

	p.list = []InstanceConfiguration{{Name: "nextcloud"}, {Name: "plex"}}
	p.inspect = map[string]InstanceInfo{
		"nextcloud": {Name: "nextcloud", AntiAffinity: []string{"streaming"}},
		"plex":      {Name: "plex"},
	}

	s.SeedAntiAffinity(ctx)

	deps, ok := s.antiAffinity.Get("streaming")
	assert.Assert(t, ok, "streaming antagonist should be registered")
	assert.Assert(t, slices.Contains(deps, "nextcloud"))

	// nextcloud disappears from the provider -> it must be pruned from the index.
	p.list = []InstanceConfiguration{{Name: "plex"}}
	s.reconcileAntiAffinityRegistry(ctx)
	assert.Assert(t, !s.hasAntiAffinity(), "index should be empty after the only dependent is removed")
}

func TestReconcileAntiAffinityRegistry_ListError(t *testing.T) {
	s, _, p := setupAntiAffinity(t)
	p.listErr = errors.New("boom")

	s.reconcileAntiAffinityRegistry(context.Background())

	assert.Assert(t, !s.hasAntiAffinity(), "a list error must leave the index untouched")
}

func TestReconcileAntiAffinityRegistry_InspectErrorSkipsInstance(t *testing.T) {
	s, _, p := setupAntiAffinity(t)
	p.list = []InstanceConfiguration{{Name: "broken"}, {Name: "ok"}}
	p.inspectErr = map[string]error{"broken": errors.New("cannot inspect")}
	p.inspect = map[string]InstanceInfo{"ok": {Name: "ok", AntiAffinity: []string{"g"}}}

	s.reconcileAntiAffinityRegistry(context.Background())

	deps, ok := s.antiAffinity.Get("g")
	assert.Assert(t, ok)
	assert.DeepEqual(t, deps, []string{"ok"})
}

func TestHandleAntiAffinityEvent(t *testing.T) {
	t.Run("created syncs and enforces", func(t *testing.T) {
		s, st, p := setupAntiAffinity(t)
		ctx := context.Background()
		s.SetGroups(map[string][]string{"streaming": {"plex"}})
		st.session("plex")      // antagonist active
		st.session("nextcloud") // dependent running

		s.handleAntiAffinityEvent(ctx, InstanceEvent{
			Type: provider.InstanceEventCreated,
			Info: InstanceInfo{Name: "nextcloud", AntiAffinity: []string{"streaming"}},
		})

		assert.Assert(t, slices.Contains(p.snapshotStopped(), "nextcloud"))
		assert.Assert(t, s.isSuppressed("nextcloud"))
	})

	t.Run("removed clears registry and suppressed set", func(t *testing.T) {
		s, _, _ := setupAntiAffinity(t)
		s.SyncInstanceAntiAffinity("nextcloud", []string{"streaming"})
		s.affinityMu.Lock()
		s.suppressed["nextcloud"] = struct{}{}
		s.affinityMu.Unlock()

		s.handleAntiAffinityEvent(context.Background(), InstanceEvent{
			Type: provider.InstanceEventRemoved,
			Info: InstanceInfo{Name: "nextcloud"},
		})

		assert.Assert(t, !s.hasAntiAffinity())
		assert.Assert(t, !s.isSuppressed("nextcloud"))
	})

	t.Run("empty name is ignored", func(t *testing.T) {
		s, _, _ := setupAntiAffinity(t)
		s.handleAntiAffinityEvent(context.Background(), InstanceEvent{
			Type: provider.InstanceEventCreated,
			Info: InstanceInfo{Name: ""},
		})
		assert.Assert(t, !s.hasAntiAffinity())
	})

	t.Run("unrelated event type is ignored", func(t *testing.T) {
		s, _, _ := setupAntiAffinity(t)
		s.handleAntiAffinityEvent(context.Background(), InstanceEvent{
			Type: provider.InstanceEventStarted,
			Info: InstanceInfo{Name: "nextcloud", AntiAffinity: []string{"streaming"}},
		})
		assert.Assert(t, !s.hasAntiAffinity(), "only created/updated/removed touch the index")
	})
}

func TestSuppressForAntiAffinity_Errors(t *testing.T) {
	t.Run("skips when instance has no session", func(t *testing.T) {
		s, _, p := setupAntiAffinity(t)
		s.affinityMu.Lock()
		s.suppressForAntiAffinity(context.Background(), "nextcloud")
		s.affinityMu.Unlock()
		assert.Equal(t, len(p.snapshotStopped()), 0)
		assert.Assert(t, !s.isSuppressed("nextcloud"))
	})

	t.Run("skips on store read error", func(t *testing.T) {
		s, st, p := setupAntiAffinity(t)
		st.getErr = errors.New("store down")
		s.affinityMu.Lock()
		s.suppressForAntiAffinity(context.Background(), "nextcloud")
		s.affinityMu.Unlock()
		assert.Equal(t, len(p.snapshotStopped()), 0)
		assert.Assert(t, !s.isSuppressed("nextcloud"))
	})

	t.Run("does not record when stop fails", func(t *testing.T) {
		s, st, p := setupAntiAffinity(t)
		st.session("nextcloud")
		p.stopErr = errors.New("cannot stop")
		s.affinityMu.Lock()
		s.suppressForAntiAffinity(context.Background(), "nextcloud")
		s.affinityMu.Unlock()
		assert.Assert(t, slices.Contains(p.snapshotStopped(), "nextcloud"), "stop was attempted")
		assert.Assert(t, !s.isSuppressed("nextcloud"), "a failed stop must not be recorded as suppressed")
	})

	t.Run("still records when session delete fails", func(t *testing.T) {
		s, st, _ := setupAntiAffinity(t)
		st.session("nextcloud")
		st.deleteErr = errors.New("cannot delete")
		s.affinityMu.Lock()
		s.suppressForAntiAffinity(context.Background(), "nextcloud")
		s.affinityMu.Unlock()
		assert.Assert(t, s.isSuppressed("nextcloud"), "the instance was stopped, so it is suppressed even if the session delete failed")
	})
}

func TestRestoreFromAntiAffinity(t *testing.T) {
	t.Run("success clears suppressed and re-establishes a tracked session", func(t *testing.T) {
		s, st, p := setupAntiAffinity(t)
		s.affinityMu.Lock()
		s.suppressed["nextcloud"] = struct{}{}
		s.restoreFromAntiAffinity(context.Background(), "nextcloud")
		s.affinityMu.Unlock()
		assert.Assert(t, slices.Contains(p.snapshotStarted(), "nextcloud"))
		assert.Assert(t, !s.isSuppressed("nextcloud"))
		// A tracked session must exist so the instance is not treated as
		// externally started and expires on the normal schedule.
		_, err := st.Get(context.Background(), "nextcloud")
		assert.NilError(t, err, "restored instance should have a tracked session")
	})

	t.Run("start failure keeps it suppressed and creates no session", func(t *testing.T) {
		s, st, p := setupAntiAffinity(t)
		p.startErr = errors.New("cannot start")
		s.affinityMu.Lock()
		s.suppressed["nextcloud"] = struct{}{}
		s.restoreFromAntiAffinity(context.Background(), "nextcloud")
		s.affinityMu.Unlock()
		assert.Assert(t, slices.Contains(p.snapshotStarted(), "nextcloud"), "start was attempted")
		assert.Assert(t, s.isSuppressed("nextcloud"), "a failed restore stays suppressed so a later reconcile retries")
		_, err := st.Get(context.Background(), "nextcloud")
		assert.ErrorIs(t, err, store.ErrKeyNotFound, "a failed restore must not create a session")
	})

	t.Run("seeds a minimal session when inspect fails", func(t *testing.T) {
		s, st, _ := setupAntiAffinity(t)
		p := s.provider.(*fakeAAProvider)
		p.inspectErr = map[string]error{"nextcloud": errors.New("cannot inspect")}
		s.affinityMu.Lock()
		s.suppressed["nextcloud"] = struct{}{}
		s.restoreFromAntiAffinity(context.Background(), "nextcloud")
		s.affinityMu.Unlock()
		got, err := st.Get(context.Background(), "nextcloud")
		assert.NilError(t, err, "a session should be seeded even when inspect fails")
		assert.Equal(t, got.Status, InstanceStatusStarting)
		assert.Assert(t, !s.isSuppressed("nextcloud"))
	})

	t.Run("still clears suppression when the session put fails", func(t *testing.T) {
		s, st, p := setupAntiAffinity(t)
		st.putErr = errors.New("store down")
		s.affinityMu.Lock()
		s.suppressed["nextcloud"] = struct{}{}
		s.restoreFromAntiAffinity(context.Background(), "nextcloud")
		s.affinityMu.Unlock()
		assert.Assert(t, slices.Contains(p.snapshotStarted(), "nextcloud"))
		assert.Assert(t, !s.isSuppressed("nextcloud"), "the instance was started, so suppression is cleared even if the session put failed")
	})
}

func TestSeedAntiAffinity_EnforcesImmediately(t *testing.T) {
	s, st, p := setupAntiAffinity(t)
	ctx := context.Background()

	p.list = []InstanceConfiguration{{Name: "plex"}, {Name: "nextcloud"}}
	p.inspect = map[string]InstanceInfo{
		"nextcloud": {Name: "nextcloud", AntiAffinity: []string{"streaming"}},
		"plex":      {Name: "plex"},
	}
	// plex (the streaming group) is active via a persisted session, and nextcloud
	// is already running — exactly the startup conflict seeding must resolve.
	s.SetGroups(map[string][]string{"streaming": {"plex"}})
	st.session("plex")
	st.session("nextcloud")

	s.SeedAntiAffinity(ctx)

	assert.Assert(t, slices.Contains(p.snapshotStopped(), "nextcloud"),
		"seeding should build the index and immediately suppress the conflicting instance")
	assert.Assert(t, s.isSuppressed("nextcloud"))
}

func TestIsGroupActive(t *testing.T) {
	t.Run("unknown group is inactive", func(t *testing.T) {
		s, _, _ := setupAntiAffinity(t)
		assert.Assert(t, !s.isGroupActive(context.Background(), "nope"))
	})

	t.Run("active when a member has a session", func(t *testing.T) {
		s, st, _ := setupAntiAffinity(t)
		s.SetGroups(map[string][]string{"streaming": {"plex"}})
		st.session("plex")
		assert.Assert(t, s.isGroupActive(context.Background(), "streaming"))
	})

	t.Run("store error is treated as inactive", func(t *testing.T) {
		s, st, _ := setupAntiAffinity(t)
		s.SetGroups(map[string][]string{"streaming": {"plex"}})
		st.getErr = errors.New("store down")
		assert.Assert(t, !s.isGroupActive(context.Background(), "streaming"))
	})
}

func TestTriggerAntiAffinityReconcile(t *testing.T) {
	t.Run("no-op without any anti-affinity", func(t *testing.T) {
		s, st, p := setupAntiAffinity(t)
		s.SetGroups(map[string][]string{"streaming": {"plex"}})
		st.session("plex")

		s.triggerAntiAffinityReconcile(context.Background())
		// No goroutine should have been spawned; give any stray one a moment.
		time.Sleep(20 * time.Millisecond)
		assert.Equal(t, len(p.snapshotStopped()), 0)
	})

	t.Run("enforces in the background when configured", func(t *testing.T) {
		s, st, p := setupAntiAffinity(t)
		s.SetGroups(map[string][]string{"streaming": {"plex"}})
		s.SyncInstanceAntiAffinity("nextcloud", []string{"streaming"})
		st.session("plex")
		st.session("nextcloud")

		s.triggerAntiAffinityReconcile(context.Background())

		assert.Assert(t, eventually(func() bool {
			return slices.Contains(p.snapshotStopped(), "nextcloud")
		}), "the background reconcile should have suppressed nextcloud")
	})
}

func TestAntiAffinityHold(t *testing.T) {
	t.Run("held while antagonist is active", func(t *testing.T) {
		s, st, _ := setupAntiAffinity(t)
		s.SetGroups(map[string][]string{"streaming": {"plex"}})
		s.SyncInstanceAntiAffinity("nextcloud", []string{"streaming"})
		st.session("plex")
		assert.Equal(t, s.antiAffinityHold(context.Background(), "nextcloud"), "streaming")
	})

	t.Run("free once no antagonist is active", func(t *testing.T) {
		s, _, _ := setupAntiAffinity(t)
		s.SetGroups(map[string][]string{"streaming": {"plex"}})
		s.SyncInstanceAntiAffinity("nextcloud", []string{"streaming"})
		assert.Equal(t, s.antiAffinityHold(context.Background(), "nextcloud"), "")
	})

	t.Run("free when instance has no anti-affinity", func(t *testing.T) {
		s, st, _ := setupAntiAffinity(t)
		s.SetGroups(map[string][]string{"streaming": {"plex"}})
		st.session("plex")
		assert.Equal(t, s.antiAffinityHold(context.Background(), "plex"), "")
	})
}

func TestInstanceRequest_HeldByAntiAffinity(t *testing.T) {
	s, st, p := setupAntiAffinity(t)
	ctx := context.Background()
	s.SetGroups(map[string][]string{"streaming": {"plex"}})
	s.SyncInstanceAntiAffinity("nextcloud", []string{"streaming"})
	st.session("plex") // antagonist active

	info, err := s.instanceRequest(ctx, "nextcloud", time.Minute, false)
	assert.NilError(t, err)

	assert.Equal(t, info.Status, InstanceStatusNotReady)
	assert.Assert(t, !info.IsReady(), "a held instance must not be reported ready")
	assert.Assert(t, strings.Contains(info.Message, "streaming"), "message should name the active group, got %q", info.Message)

	// It must not have been started, and no session should have been created.
	assert.Equal(t, len(p.snapshotStarted()), 0, "held instance must not be started")
	_, getErr := st.Get(ctx, "nextcloud")
	assert.ErrorIs(t, getErr, store.ErrKeyNotFound, "held instance must not get a session")
}

func TestRequestReadySession_TimeoutReportsAntiAffinityHold(t *testing.T) {
	s, st, _ := setupAntiAffinity(t)
	s.BlockingRefreshFrequency = 10 * time.Millisecond
	s.SetGroups(map[string][]string{"streaming": {"plex"}})
	s.SyncInstanceAntiAffinity("nextcloud", []string{"streaming"})
	st.session("plex") // antagonist active -> nextcloud is held

	_, err := s.requestReadySession(context.Background(), []string{"nextcloud"}, time.Minute, 60*time.Millisecond, false)

	var te ErrTimeout
	assert.Assert(t, errors.As(err, &te), "expected ErrTimeout, got %v", err)
	assert.Equal(t, len(te.Instances), 1)
	assert.Equal(t, te.Instances[0].Instance.Name, "nextcloud")
	assert.Assert(t, strings.Contains(te.Instances[0].Instance.Message, "streaming"),
		"held instance message should name the antagonist group, got %q", te.Instances[0].Instance.Message)
	assert.Assert(t, strings.Contains(te.Error(), "streaming"), "timeout error should carry the reason")
}

// eventually polls cond for up to a second, returning true as soon as it holds.
func eventually(cond func() bool) bool {
	for i := 0; i < 100; i++ {
		if cond() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return cond()
}
