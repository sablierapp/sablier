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
	"testing/synctest"
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
	// failure that is not ErrKeyNotFound). deleteErr behaves likewise for Delete.
	getErr    error
	deleteErr error
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
	// onStart, when set, runs at the end of InstanceStart (outside mu).
	onStart func(name string)
	// list/listErr back InstanceList; inspect/inspectErr back InstanceInspect
	// (keyed by instance name). Unset inspect entries fall back to a bare
	// InstanceInfo{Name: name}.
	list       []InstanceConfiguration
	listErr    error
	inspect    map[string]InstanceInfo
	inspectErr map[string]error
	// inspected records every InstanceInspect call, in order.
	inspected []string
}

func (f *fakeAAProvider) InstanceStart(_ context.Context, name string) error {
	f.mu.Lock()
	f.started = append(f.started, name)
	err, onStart := f.startErr, f.onStart
	f.mu.Unlock()
	if onStart != nil {
		onStart(name)
	}
	return err
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
	f.inspected = append(f.inspected, name)
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

func (f *fakeAAProvider) snapshotInspected() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.inspected)
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
	synctest.Test(t, func(t *testing.T) {
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

		// The restore goes through the request path, which starts asynchronously.
		synctest.Wait()
		assert.Assert(t, slices.Contains(p.snapshotStarted(), "nextcloud"), "dependent should be restored")
		assert.Assert(t, !s.isSuppressed("nextcloud"), "dependent should no longer be suppressed")
	})
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
	synctest.Test(t, func(t *testing.T) {
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

		// All antagonists inactive -> restore (asynchronously, via the request path).
		_ = st.Delete(ctx, "plex")
		s.reconcileAntiAffinity(ctx)
		synctest.Wait()
		assert.Assert(t, slices.Contains(p.snapshotStarted(), "nextcloud"),
			"dependent should be restored once all antagonists are inactive")
		assert.Assert(t, !s.isSuppressed("nextcloud"))
	})
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

	p.list = []InstanceConfiguration{{Name: "nextcloud", AntiAffinity: []string{"streaming"}}, {Name: "plex"}}

	s.SeedAntiAffinity(ctx)

	deps, ok := s.antiAffinity.Get("streaming")
	assert.Assert(t, ok, "streaming antagonist should be registered")
	assert.Assert(t, slices.Contains(deps, "nextcloud"))

	// nextcloud disappears from the provider -> it must be pruned from the index.
	p.list = []InstanceConfiguration{{Name: "plex"}}
	s.reconcileAntiAffinityRegistry(ctx)
	assert.Assert(t, !s.hasAntiAffinity(), "index should be empty after the only dependent is removed")
}

// An instance that is still listed but no longer declares an anti-affinity
// (its label was removed) must lose its registry entry, not keep a stale one.
func TestReconcileAntiAffinityRegistry_ClearsRemovedLabel(t *testing.T) {
	s, _, p := setupAntiAffinity(t)
	ctx := context.Background()

	p.list = []InstanceConfiguration{
		{Name: "nextcloud", AntiAffinity: []string{"streaming"}},
		{Name: "immich", AntiAffinity: []string{"streaming", "gaming"}},
	}
	s.reconcileAntiAffinityRegistry(ctx)

	p.list = []InstanceConfiguration{
		{Name: "nextcloud"},
		{Name: "immich", AntiAffinity: []string{"gaming"}},
	}
	s.reconcileAntiAffinityRegistry(ctx)

	_, ok := s.antiAffinity.Get("streaming")
	assert.Assert(t, !ok, "streaming should have no dependents left")
	deps, ok := s.antiAffinity.Get("gaming")
	assert.Assert(t, ok)
	assert.DeepEqual(t, deps, []string{"immich"})
	assert.Equal(t, len(s.antiAffinity.GroupsOf("nextcloud")), 0)
}

func TestReconcileAntiAffinityRegistry_ListError(t *testing.T) {
	s, _, p := setupAntiAffinity(t)
	p.listErr = errors.New("boom")

	s.reconcileAntiAffinityRegistry(context.Background())

	assert.Assert(t, !s.hasAntiAffinity(), "a list error must leave the index untouched")
}

func TestReconcileAntiAffinityRegistry_UsesListWithoutInspect(t *testing.T) {
	s, _, p := setupAntiAffinity(t)
	p.list = []InstanceConfiguration{
		{Name: "nextcloud", AntiAffinity: []string{"streaming", "gaming"}},
		{Name: "immich", AntiAffinity: []string{"streaming"}},
		{Name: "plex"},
	}
	// Inspect results that disagree with the list must not be used.
	p.inspect = map[string]InstanceInfo{"plex": {Name: "plex", AntiAffinity: []string{"other"}}}

	// Only the rebuild is covered: enforcing the registry afterwards may
	// legitimately inspect instances.
	s.reconcileAntiAffinityRegistry(context.Background())

	assert.Assert(t, len(p.snapshotInspected()) == 0, "the registry rebuild must not inspect instances, got %v", p.snapshotInspected())
	deps, ok := s.antiAffinity.Get("streaming")
	assert.Assert(t, ok)
	slices.Sort(deps)
	assert.DeepEqual(t, deps, []string{"immich", "nextcloud"})
	deps, ok = s.antiAffinity.Get("gaming")
	assert.Assert(t, ok)
	assert.DeepEqual(t, deps, []string{"nextcloud"})
	_, ok = s.antiAffinity.Get("other")
	assert.Assert(t, !ok, "inspect data leaked into the registry")
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
	t.Run("requests the instance, tracking a session and clearing suppression", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			s, st, p := setupAntiAffinity(t)
			s.affinityMu.Lock()
			s.suppressed["nextcloud"] = struct{}{}
			s.restoreFromAntiAffinity(context.Background(), "nextcloud")
			s.affinityMu.Unlock()

			// Requesting establishes a tracked session synchronously (so the instance
			// is not treated as externally started and expires normally) and starts
			// the instance via the normal, asynchronous request path.
			_, err := st.Get(context.Background(), "nextcloud")
			assert.NilError(t, err, "restored instance should have a tracked session")
			assert.Assert(t, !s.isSuppressed("nextcloud"))
			synctest.Wait()
			assert.Assert(t, slices.Contains(p.snapshotStarted(), "nextcloud"), "restored instance should be started")
		})
	})

	t.Run("keeps it suppressed when the request fails", func(t *testing.T) {
		s, st, p := setupAntiAffinity(t)
		st.getErr = errors.New("store down") // makes instanceRequest fail before starting
		s.affinityMu.Lock()
		s.suppressed["nextcloud"] = struct{}{}
		s.restoreFromAntiAffinity(context.Background(), "nextcloud")
		s.affinityMu.Unlock()
		assert.Assert(t, s.isSuppressed("nextcloud"), "a failed restore stays suppressed so a later reconcile retries")
		assert.Equal(t, len(p.snapshotStarted()), 0, "no start should be dispatched when the request fails")
	})
}

func TestSeedAntiAffinity_EnforcesImmediately(t *testing.T) {
	s, st, p := setupAntiAffinity(t)
	ctx := context.Background()

	p.list = []InstanceConfiguration{{Name: "plex"}, {Name: "nextcloud", AntiAffinity: []string{"streaming"}}}
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
		synctest.Test(t, func(t *testing.T) {
			s, st, p := setupAntiAffinity(t)
			s.SetGroups(map[string][]string{"streaming": {"plex"}})
			st.session("plex")

			s.triggerAntiAffinityReconcile(context.Background())
			// No goroutine should have been spawned. Wait until a stray one, if any, is done.
			synctest.Wait()
			assert.Equal(t, len(p.snapshotStopped()), 0)
		})
	})

	t.Run("enforces in the background when configured", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			s, st, p := setupAntiAffinity(t)
			s.SetGroups(map[string][]string{"streaming": {"plex"}})
			s.SyncInstanceAntiAffinity("nextcloud", []string{"streaming"})
			st.session("plex")
			st.session("nextcloud")

			s.triggerAntiAffinityReconcile(context.Background())

			synctest.Wait()
			assert.Assert(t, slices.Contains(p.snapshotStopped(), "nextcloud"),
				"the background reconcile should have suppressed nextcloud")
		})
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

// A held instance whose session outlived a stop must not be started again by
// a retry of its failed start.
func TestInstanceRequest_HeldByAntiAffinity_WithStoppedSession(t *testing.T) {
	s, st, p := setupAntiAffinity(t)
	ctx := context.Background()
	s.SetGroups(map[string][]string{"streaming": {"plex"}})
	s.SyncInstanceAntiAffinity("nextcloud", []string{"streaming"})
	st.session("plex") // antagonist active
	_ = st.Put(ctx, InstanceInfo{Name: "nextcloud", Status: InstanceStatusStarting}, time.Minute)
	s.recordFailedStart("nextcloud")
	p.inspect = map[string]InstanceInfo{"nextcloud": {Name: "nextcloud", Status: InstanceStatusStopped}}

	info, err := s.instanceRequest(ctx, "nextcloud", time.Minute, false)
	assert.NilError(t, err)
	assert.Equal(t, info.Status, InstanceStatusNotReady)
	assert.Equal(t, len(p.snapshotStarted()), 0, "held instance must not be started")
}

// recordFailedStart marks name as having a failed start whose retry is due.
func (s *Sablier) recordFailedStart(name string) {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	s.failedStarts[name] = &failedStart{failures: 1}
}

func (s *Sablier) hasFailedStart(name string) bool {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	_, ok := s.failedStarts[name]
	return ok
}

// Suppression forgets a failed start, so a request racing with it (one that
// passed antiAffinityHold before the antagonist became active) does not start
// the instance again behind the suppression's back.
func TestSuppressForAntiAffinity_ForgetsFailedStart(t *testing.T) {
	s, st, p := setupAntiAffinity(t)
	ctx := context.Background()
	s.SetGroups(map[string][]string{"streaming": {"plex"}})
	s.SyncInstanceAntiAffinity("nextcloud", []string{"streaming"})
	_ = st.Put(ctx, InstanceInfo{Name: "nextcloud", Status: InstanceStatusStarting}, time.Minute)
	s.recordFailedStart("nextcloud")
	st.session("plex")

	s.reconcileAntiAffinity(ctx)
	assert.Assert(t, s.isSuppressed("nextcloud"))
	assert.Assert(t, !s.hasFailedStart("nextcloud"))

	// The racing request re-puts the session after the suppression deleted it.
	_ = st.Put(ctx, InstanceInfo{Name: "nextcloud", Status: InstanceStatusStarting}, time.Minute)
	_, ok := s.retryFailedStart(ctx, "nextcloud", InstanceInfo{Name: "nextcloud"}, false)
	assert.Assert(t, !ok, "a suppressed instance must not be started again")
	assert.Equal(t, len(p.snapshotStarted()), 0)
}

// A retry dispatched just before an antagonist became active forces the
// instance idle again once its start lands, since the suppression may already
// have run and will not look at it again.
func TestRetryFailedStart_ForcedIdleWhenAntiAffinityBecameActive(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, st, p := setupAntiAffinity(t)
		ctx := context.Background()
		s.SetGroups(map[string][]string{"streaming": {"plex"}})
		s.SyncInstanceAntiAffinity("nextcloud", []string{"streaming"})
		_ = st.Put(ctx, InstanceInfo{Name: "nextcloud", Status: InstanceStatusStarting}, time.Minute)
		s.recordFailedStart("nextcloud")
		// The antagonist becomes active while nextcloud is starting.
		p.onStart = func(string) { st.session("plex") }

		info, err := s.instanceRequest(ctx, "nextcloud", time.Minute, false)
		assert.NilError(t, err)
		assert.Equal(t, info.Status, InstanceStatusStarting)
		synctest.Wait()

		assert.DeepEqual(t, p.snapshotStarted(), []string{"nextcloud"})
		assert.DeepEqual(t, p.snapshotStopped(), []string{"nextcloud"})
	})
}

// A retry whose session is removed while it is starting (a stop event, an
// expiration or a suppression) stops the instance again, so it is not left
// running with no session to ever stop it.
func TestRetryFailedStart_StoppedWhenSessionRemovedMeanwhile(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, st, p := setupAntiAffinity(t)
		ctx := context.Background()
		_ = st.Put(ctx, InstanceInfo{Name: "nextcloud", Status: InstanceStatusStarting}, time.Minute)
		s.recordFailedStart("nextcloud")
		// The request re-puts the session right after dispatching the retry,
		// so the removal is simulated once the start has begun.
		started := make(chan struct{})
		removed := make(chan struct{})
		p.onStart = func(string) {
			close(started)
			<-removed
		}

		_, err := s.instanceRequest(ctx, "nextcloud", time.Minute, false)
		assert.NilError(t, err)
		<-started
		assert.NilError(t, s.RemoveInstance(ctx, "nextcloud"))
		close(removed)
		synctest.Wait()

		assert.DeepEqual(t, p.snapshotStopped(), []string{"nextcloud"})
	})
}

// A successful retry with its session in place leaves the instance running.
func TestRetryFailedStart_KeepsRunningWithSession(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, st, p := setupAntiAffinity(t)
		ctx := context.Background()
		_ = st.Put(ctx, InstanceInfo{Name: "nextcloud", Status: InstanceStatusStarting}, time.Minute)
		s.recordFailedStart("nextcloud")

		_, err := s.instanceRequest(ctx, "nextcloud", time.Minute, false)
		assert.NilError(t, err)
		synctest.Wait()

		assert.DeepEqual(t, p.snapshotStarted(), []string{"nextcloud"})
		assert.Equal(t, len(p.snapshotStopped()), 0)
		assert.Assert(t, !s.hasFailedStart("nextcloud"))
	})
}

// A stop from outside Sablier (an InstanceStopped event) forgets a failed
// start, so a request arriving before the session is gone does not undo it.
func TestRemoveInstance_ForgetsFailedStart(t *testing.T) {
	s, _, _ := setupAntiAffinity(t)
	s.recordFailedStart("nextcloud")
	assert.NilError(t, s.RemoveInstance(context.Background(), "nextcloud"))
	assert.Assert(t, !s.hasFailedStart("nextcloud"))
}

func TestOnInstanceExpired_ForgetsFailedStart(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, _, _ := setupAntiAffinity(t)
		s.recordFailedStart("nextcloud")
		s.OnInstanceExpired(context.Background())("nextcloud")
		synctest.Wait()
		assert.Assert(t, !s.hasFailedStart("nextcloud"))
	})
}

// Review on #1240: request R retries and stores Starting, while request C,
// which read the session as Ready (scale mode at its idle size) before R's
// Put, writes Ready back. R's retry fails. Its finished pending entry must not
// block the next retry, or the instance stays idle for the rest of the session.
func TestRetryFailedStart_FinishedFailedRetryDoesNotBlockNextRetry(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, st, p := setupAntiAffinity(t)
		ctx := context.Background()
		ready := InstanceInfo{Name: "hub", Status: InstanceStatusReady, CurrentReplicas: 1, DesiredReplicas: 2}
		_ = st.Put(ctx, ready, time.Minute)
		s.recordFailedStart("hub")
		p.startErr = errors.New("quota exceeded")

		// R: retries, and its start fails.
		_, err := s.instanceRequest(ctx, "hub", time.Minute, false)
		assert.NilError(t, err)
		synctest.Wait()
		// C: puts the Ready it read before R's Put back over R's Starting.
		_ = st.Put(ctx, ready, time.Minute)
		assert.Equal(t, len(p.snapshotStarted()), 1)

		// After the backoff (4s after a second failure) the next request retries.
		time.Sleep(4 * time.Second)
		p.mu.Lock()
		p.startErr = nil
		p.mu.Unlock()
		info, err := s.instanceRequest(ctx, "hub", time.Minute, false)
		assert.NilError(t, err)
		assert.Equal(t, info.Status, InstanceStatusStarting)
		synctest.Wait()
		assert.DeepEqual(t, p.snapshotStarted(), []string{"hub", "hub"})
		assert.Assert(t, !s.hasFailedStart("hub"))
	})
}

// A retry that fails, but may still bring the workload up later (e.g. after
// InstanceStartTimeout), is undone when the instance was stopped on purpose in
// the meantime, and its failure is not recorded again.
func TestRetryFailedStart_StoppedDuringFailedRetry_StopsAndRecordsNoFailure(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, st, p := setupAntiAffinity(t)
		ctx := context.Background()
		_ = st.Put(ctx, InstanceInfo{Name: "hub", Status: InstanceStatusStarting}, time.Minute)
		s.recordFailedStart("hub")
		p.startErr = errors.New("timed out waiting for dependencies")
		// A stop event arrives while the retry is in flight.
		p.onStart = func(name string) { _ = s.RemoveInstance(ctx, name) }

		_, err := s.instanceRequest(ctx, "hub", time.Minute, false)
		assert.NilError(t, err)
		synctest.Wait()

		assert.DeepEqual(t, p.snapshotStopped(), []string{"hub"})
		assert.Assert(t, !s.hasFailedStart("hub"), "a start stopped on purpose must not be retried")
	})
}

// The same undo applies to a first start, not only to retries.
func TestFirstStart_StoppedDuringStart_StopsAgain(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, _, p := setupAntiAffinity(t)
		ctx := context.Background()
		started := make(chan struct{})
		release := make(chan struct{})
		p.onStart = func(string) {
			close(started)
			<-release
		}

		_, err := s.instanceRequest(ctx, "hub", time.Minute, false)
		assert.NilError(t, err)
		<-started
		assert.NilError(t, s.RemoveInstance(ctx, "hub"))
		close(release)
		synctest.Wait()

		assert.DeepEqual(t, p.snapshotStopped(), []string{"hub"})
	})
}

// Review on #1240: a first start (not only a retry) can land after a
// suppression already stopped the instance. The suppression does not look at
// it again, so the start stops it again once it lands.
func TestFirstStart_SuppressedWhileStarting_StopsAgain(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, st, p := setupAntiAffinity(t)
		ctx := context.Background()
		s.SetGroups(map[string][]string{"streaming": {"plex"}})
		s.SyncInstanceAntiAffinity("nextcloud", []string{"streaming"})
		started := make(chan struct{})
		land := make(chan struct{})
		p.onStart = func(string) {
			close(started)
			<-land
		}

		_, err := s.instanceRequest(ctx, "nextcloud", time.Minute, false)
		assert.NilError(t, err)
		<-started
		// The antagonist becomes active and the suppression runs before the
		// start has landed.
		st.session("plex")
		s.reconcileAntiAffinity(ctx)
		assert.Assert(t, s.isSuppressed("nextcloud"))
		assert.Equal(t, len(p.snapshotStopped()), 1)

		close(land)
		synctest.Wait()
		assert.Equal(t, len(p.snapshotStopped()), 2, "the start must stop the instance again once it lands")
	})
}

// A suppression whose stop fails keeps the session, so it must keep the
// recorded failure too, or the failed start is never retried.
func TestSuppressForAntiAffinity_StopFails_KeepsFailedStart(t *testing.T) {
	s, st, p := setupAntiAffinity(t)
	ctx := context.Background()
	s.SetGroups(map[string][]string{"streaming": {"plex"}})
	s.SyncInstanceAntiAffinity("nextcloud", []string{"streaming"})
	_ = st.Put(ctx, InstanceInfo{Name: "nextcloud", Status: InstanceStatusStarting}, time.Minute)
	s.recordFailedStart("nextcloud")
	p.stopErr = errors.New("forbidden")
	st.session("plex")

	s.reconcileAntiAffinity(ctx)

	assert.Assert(t, !s.isSuppressed("nextcloud"))
	assert.Assert(t, s.hasFailedStart("nextcloud"))
}

// A start reported failed that came up after all (e.g. it hit
// InstanceStartTimeout just before becoming ready) is not retried once an
// inspect shows it fully started. In scale mode only the active replica count
// counts as started, since the idle profile also reports ready.
func TestInstanceRequest_FullyStartedClearsFailedStart(t *testing.T) {
	scale := &ScaleConfig{Idle: ResourceProfile{Replicas: 1}, Active: ResourceProfile{Replicas: 2}}
	for _, tc := range []struct {
		name    string
		info    InstanceInfo
		cleared bool
	}{
		{"ready", InstanceInfo{Status: InstanceStatusReady, CurrentReplicas: 1, DesiredReplicas: 1}, true},
		{"starting", InstanceInfo{Status: InstanceStatusStarting, DesiredReplicas: 1}, false},
		{"scale mode at idle size", InstanceInfo{Status: InstanceStatusReady, CurrentReplicas: 1, DesiredReplicas: 2, ScaleConfig: scale}, false},
		{"scale mode at active size", InstanceInfo{Status: InstanceStatusReady, CurrentReplicas: 2, DesiredReplicas: 2, ScaleConfig: scale}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, st, p := setupAntiAffinity(t)
			ctx := context.Background()
			_ = st.Put(ctx, InstanceInfo{Name: "hub", Status: InstanceStatusStarting}, time.Minute)
			s.pendingMu.Lock()
			s.failedStarts["hub"] = &failedStart{failures: 1, retryAt: time.Now().Add(time.Hour)}
			s.pendingMu.Unlock()
			info := tc.info
			info.Name = "hub"
			p.inspect = map[string]InstanceInfo{"hub": info}

			_, err := s.instanceRequest(ctx, "hub", time.Minute, false)
			assert.NilError(t, err)
			assert.Equal(t, !s.hasFailedStart("hub"), tc.cleared)
		})
	}
}

// With unlabeled requests rejected, a retry first checks that the instance
// still opts in: one whose sablier.enable was removed during the session is
// not started again.
func TestRetryFailedStart_RejectUnlabeled(t *testing.T) {
	for _, tc := range []struct {
		enabled string
		starts  int
	}{{"true", 1}, {"", 0}} {
		t.Run("enable="+tc.enabled, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				s, st, p := setupAntiAffinity(t)
				ctx := context.Background()
				_ = st.Put(ctx, InstanceInfo{Name: "hub", Status: InstanceStatusStarting, Enabled: "true"}, time.Minute)
				s.recordFailedStart("hub")
				p.inspect = map[string]InstanceInfo{"hub": {Name: "hub", Status: InstanceStatusStopped, Enabled: tc.enabled}}

				_, err := s.instanceRequest(ctx, "hub", time.Minute, true)
				assert.NilError(t, err)
				synctest.Wait()
				assert.Equal(t, len(p.snapshotStarted()), tc.starts)
				if tc.starts == 0 {
					assert.Assert(t, !s.hasFailedStart("hub"))
				}
			})
		})
	}
}

func TestRequestReadySession_TimeoutReportsAntiAffinityHold(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
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
	})
}
