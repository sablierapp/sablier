package sablier_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/provider/providertest"
	"github.com/sablierapp/sablier/pkg/sablier"
	"go.uber.org/mock/gomock"
	"gotest.tools/v3/assert"
)

const activityInterval = 20 * time.Second

var plexPorts = []sablier.Port{{Number: 32400, Proto: "tcp"}}

// fakeSource records Watch and Unwatch calls in order and hands out queued
// Drain results, then empty ones.
type fakeSource struct {
	mu        sync.Mutex
	calls     []string
	ports     map[string][]sablier.Port
	drains    []map[string]uint64
	watchErrs []error // returned by the next Watch calls, in order
}

func newFakeSource(drains ...map[string]uint64) *fakeSource {
	return &fakeSource{ports: map[string][]sablier.Port{}, drains: drains}
}

func (f *fakeSource) Watch(_ context.Context, instance string, ports []sablier.Port) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "watch:"+instance)
	f.ports[instance] = ports
	if len(f.watchErrs) > 0 {
		err := f.watchErrs[0]
		f.watchErrs = f.watchErrs[1:]
		return err
	}
	return nil
}

func (f *fakeSource) Unwatch(_ context.Context, instance string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "unwatch:"+instance)
	return nil
}

func (f *fakeSource) Drain(context.Context) (map[string]uint64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.drains) == 0 {
		return map[string]uint64{}, nil
	}
	d := f.drains[0]
	f.drains = f.drains[1:]
	return d, nil
}

func (f *fakeSource) snapshot() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.calls)
}

func (f *fakeSource) watchedPorts(instance string) []sablier.Port {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.ports[instance]
}

// activityInstance is a running, Sablier-managed instance that opted into
// activity tracking with the given configuration.
func activityInstance(name string, activity sablier.ActivityConfig) sablier.InstanceInfo {
	groups := activity.Groups
	if groups == nil {
		groups = []string{"default"}
	}
	return sablier.InstanceInfo{
		Name:   name,
		Status: sablier.InstanceStatusReady,
		Config: &sablier.InstanceConfig{Enabled: true, Groups: groups, Activity: &activity},
	}
}

// readyInfo is the session record of a running instance.
func readyInfo(name string) sablier.InstanceInfo {
	return sablier.InstanceInfo{Name: name, Status: sablier.InstanceStatusReady}
}

// expectListings makes each reconciliation list the next listing of running
// instances, exactly once and in order. Register every expectation before
// starting the loop: gomock expectations are not safe to add concurrently.
func expectListings(p *providertest.MockProvider, listings ...[]sablier.InstanceConfiguration) {
	calls := make([]any, 0, len(listings))
	for _, running := range listings {
		calls = append(calls, p.EXPECT().InstanceList(gomock.Any(), provider.InstanceListOptions{All: false}).Return(running, nil))
	}
	gomock.InOrder(calls...)
}

// startWatchActivity runs WatchActivity on src until the test ends and
// returns the provider's event channel. The listings the reconciliations see
// are set with expectListings beforehand.
func startWatchActivity(t *testing.T, s *sablier.Sablier, p *providertest.MockProvider, src sablier.ActivitySource) chan<- sablier.InstanceEvent {
	t.Helper()
	events := make(chan sablier.InstanceEvent, 1)
	p.EXPECT().InstanceEvents(gomock.Any(), provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventStarted, provider.InstanceEventStopped, provider.InstanceEventRemoved},
	}).Return(sablier.InstanceEventStream{Events: events, Err: make(chan error)})

	done := make(chan struct{})
	go func() {
		defer close(done)
		s.WatchActivity(t.Context(), src, activityInterval)
	}()
	// t.Context is cancelled before cleanups run: wait for the loop to return.
	t.Cleanup(func() { <-done })
	synctest.Wait()
	return events
}

// started sends a started event for each instance and waits for the loop to
// handle them.
func started(events chan<- sablier.InstanceEvent, infos ...sablier.InstanceInfo) {
	for _, info := range infos {
		events <- sablier.InstanceEvent{Type: provider.InstanceEventStarted, Info: info}
		synctest.Wait()
	}
}

// nextActivityTick advances the fake clock past exactly one activity tick and
// waits for the loop to handle it.
func nextActivityTick() {
	time.Sleep(activityInterval + time.Second)
	synctest.Wait()
}

// sessionRequests returns the sorted session requests a fakeRecorder saw.
func sessionRequests(r *fakeRecorder) []string {
	var out []string
	for _, c := range r.snapshot() {
		if strings.HasPrefix(c, "session:") {
			out = append(out, c)
		}
	}
	slices.Sort(out)
	return out
}

func TestWatchActivity_StartedEventWatchesInstance(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, _, p, rec := setupSablierWithMetrics(t)
		s.ActivityReconcileInterval = time.Hour
		src := newFakeSource()
		expectListings(p, nil)
		events := startWatchActivity(t, s, p, src)

		started(events, activityInstance("plex", sablier.ActivityConfig{Ports: plexPorts, MinBytes: 1000}))

		assert.DeepEqual(t, src.snapshot(), []string{"watch:plex"})
		assert.DeepEqual(t, src.watchedPorts("plex"), plexPorts)
		assert.Assert(t, slices.Contains(rec.snapshot(), "activity_watched:1"))
	})
}

func TestWatchActivity_StartedEventIgnoresInstanceWithoutActivity(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, _, p := setupSablier(t)
		s.ActivityReconcileInterval = time.Hour
		src := newFakeSource()
		expectListings(p, nil)
		events := startWatchActivity(t, s, p, src)

		started(events,
			sablier.InstanceInfo{Name: "nginx", Config: &sablier.InstanceConfig{Enabled: true, Groups: []string{"default"}}},
			sablier.InstanceInfo{Name: "bare"}, // the provider could not inspect it
		)

		assert.Assert(t, len(src.snapshot()) == 0, "unexpected calls: %v", src.snapshot())
	})
}

func TestWatchActivity_StoppedEventUnwatches(t *testing.T) {
	for _, eventType := range []provider.InstanceEventType{provider.InstanceEventStopped, provider.InstanceEventRemoved} {
		t.Run(string(eventType), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				s, _, p, rec := setupSablierWithMetrics(t)
				s.ActivityReconcileInterval = time.Hour
				src := newFakeSource()
				expectListings(p, nil)
				events := startWatchActivity(t, s, p, src)
				started(events, activityInstance("plex", sablier.ActivityConfig{Ports: plexPorts, MinBytes: 1000}))

				events <- sablier.InstanceEvent{Type: eventType, Info: sablier.InstanceInfo{Name: "plex"}}
				synctest.Wait()

				assert.DeepEqual(t, src.snapshot(), []string{"watch:plex", "unwatch:plex"})
				calls := rec.snapshot()
				assert.Equal(t, calls[len(calls)-1], "activity_watched:0")
			})
		})
	}
}

func TestWatchActivity_StartedEventForWatchedInstanceRewatches(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, _, p := setupSablier(t)
		s.ActivityReconcileInterval = time.Hour
		src := newFakeSource()
		expectListings(p, nil)
		events := startWatchActivity(t, s, p, src)
		plex := activityInstance("plex", sablier.ActivityConfig{Ports: plexPorts, MinBytes: 1000})

		started(events, plex, plex)

		assert.DeepEqual(t, src.snapshot(), []string{"watch:plex", "unwatch:plex", "watch:plex"})
	})
}

func TestWatchActivity_SkipsSessionDurationNotLongerThanInterval(t *testing.T) {
	tests := []struct {
		name            string
		sessionDuration time.Duration
		defaultDuration time.Duration
	}{
		{name: "equal to the interval", sessionDuration: activityInterval, defaultDuration: 5 * time.Minute},
		{name: "negative", sessionDuration: -time.Minute, defaultDuration: 5 * time.Minute},
		{name: "default equal to the interval", sessionDuration: 0, defaultDuration: activityInterval},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				s, _, p := setupSablier(t)
				s.ActivityReconcileInterval = time.Hour
				s.DefaultSessionDuration = tt.defaultDuration
				src := newFakeSource()
				expectListings(p, nil)
				events := startWatchActivity(t, s, p, src)

				started(events, activityInstance("plex", sablier.ActivityConfig{Ports: plexPorts, SessionDuration: tt.sessionDuration, MinBytes: 1000}))

				assert.Assert(t, len(src.snapshot()) == 0, "unexpected calls: %v", src.snapshot())
			})
		})
	}
}

func TestWatchActivity_ReconcileWatchesMissedInstance(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, _, p := setupSablier(t)
		s.ActivityReconcileInterval = time.Hour
		src := newFakeSource()
		p.EXPECT().InstanceInspect(gomock.Any(), "plex").
			Return(activityInstance("plex", sablier.ActivityConfig{Ports: plexPorts, MinBytes: 1000}), nil)

		// nginx has no activity tracking: it is not even inspected.
		expectListings(p, []sablier.InstanceConfiguration{
			{Name: "plex", Enabled: "true", Activity: true},
			{Name: "nginx", Enabled: "true"},
		})
		startWatchActivity(t, s, p, src)

		assert.DeepEqual(t, src.snapshot(), []string{"watch:plex"})
		assert.DeepEqual(t, src.watchedPorts("plex"), plexPorts)
	})
}

func TestWatchActivity_ReconcileUnwatchesMissingInstance(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, _, p := setupSablier(t)
		s.ActivityReconcileInterval = 30 * time.Second
		src := newFakeSource()
		p.EXPECT().InstanceInspect(gomock.Any(), "plex").
			Return(activityInstance("plex", sablier.ActivityConfig{Ports: plexPorts, MinBytes: 1000}), nil)
		// plex stops and its event is lost: the next listing no longer has it.
		expectListings(p, []sablier.InstanceConfiguration{{Name: "plex", Enabled: "true", Activity: true}}, nil)
		startWatchActivity(t, s, p, src)

		time.Sleep(s.ActivityReconcileInterval + time.Second)
		synctest.Wait()

		assert.DeepEqual(t, src.snapshot(), []string{"watch:plex", "unwatch:plex"})
	})
}

func TestWatchActivity_ReconcileRetriesFailedWatch(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, _, p := setupSablier(t)
		s.ActivityReconcileInterval = 30 * time.Second
		src := newFakeSource()
		src.watchErrs = []error{errors.New("attach failed")}
		running := []sablier.InstanceConfiguration{{Name: "plex", Enabled: "true", Activity: true}}
		// Inspected by the first two reconciliations only: once watched, plex is left alone.
		p.EXPECT().InstanceInspect(gomock.Any(), "plex").
			Return(activityInstance("plex", sablier.ActivityConfig{Ports: plexPorts, MinBytes: 1000}), nil).Times(2)
		expectListings(p, running, running, running)
		startWatchActivity(t, s, p, src)

		// Two more reconciliations: the first retries the failed watch.
		time.Sleep(2*s.ActivityReconcileInterval + time.Second)
		synctest.Wait()

		assert.DeepEqual(t, src.snapshot(), []string{"watch:plex", "watch:plex"})
	})
}

func TestWatchActivity_BelowMinBytesDoesNotRefresh(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, sessions, p := setupSablier(t)
		s.ActivityReconcileInterval = time.Hour
		s.SetGroups(map[string][]string{"plex": {"plex"}})
		sessions.EXPECT().Range(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
		src := newFakeSource(map[string]uint64{"plex": 999})
		expectListings(p, nil)
		events := startWatchActivity(t, s, p, src)
		started(events, activityInstance("plex", sablier.ActivityConfig{Ports: plexPorts, MinBytes: 1000, Groups: []string{"plex"}}))

		// No sessions.Put is expected: the mock fails the test on any call.
		nextActivityTick()
	})
}

func TestWatchActivity_RefreshesEveryGroup(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, sessions, p, rec := setupSablierWithMetrics(t)
		s.ActivityReconcileInterval = time.Hour
		s.SetGroups(map[string][]string{"plex": {"plex"}, "media": {"plex", "jellyfin"}})
		sessions.EXPECT().Range(gomock.Any(), gomock.Any()).Return(nil)
		sessions.EXPECT().Get(gomock.Any(), "plex").Return(readyInfo("plex"), nil).Times(2)
		sessions.EXPECT().Put(gomock.Any(), readyInfo("plex"), 10*time.Minute).Return(nil).Times(2)
		sessions.EXPECT().Get(gomock.Any(), "jellyfin").Return(readyInfo("jellyfin"), nil)
		sessions.EXPECT().Put(gomock.Any(), readyInfo("jellyfin"), 10*time.Minute).Return(nil)
		src := newFakeSource(map[string]uint64{"plex": 1000})
		expectListings(p, nil)
		events := startWatchActivity(t, s, p, src)
		started(events, activityInstance("plex", sablier.ActivityConfig{
			Ports: plexPorts, SessionDuration: 10 * time.Minute, MinBytes: 1000, Groups: []string{"plex", "media"},
		}))

		nextActivityTick()

		assert.DeepEqual(t, sessionRequests(rec), []string{"session:activity/media", "session:activity/plex"})
	})
}

func TestWatchActivity_RefreshesGroupOnceForSeveralActiveMembers(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, sessions, p, rec := setupSablierWithMetrics(t)
		s.ActivityReconcileInterval = time.Hour
		s.SetGroups(map[string][]string{"web": {"web1", "web2"}})
		sessions.EXPECT().Range(gomock.Any(), gomock.Any()).Return(nil)
		for _, name := range []string{"web1", "web2"} {
			sessions.EXPECT().Get(gomock.Any(), name).Return(readyInfo(name), nil)
			sessions.EXPECT().Put(gomock.Any(), readyInfo(name), 10*time.Minute).Return(nil)
		}
		src := newFakeSource(map[string]uint64{"web1": 1000, "web2": 5000})
		expectListings(p, nil)
		events := startWatchActivity(t, s, p, src)
		activity := sablier.ActivityConfig{Ports: plexPorts, SessionDuration: 10 * time.Minute, MinBytes: 1000, Groups: []string{"web"}}
		started(events, activityInstance("web1", activity), activityInstance("web2", activity))

		nextActivityTick()

		assert.DeepEqual(t, sessionRequests(rec), []string{"session:activity/web"})
	})
}

func TestWatchActivity_ImplicitDefaultGroupRefreshesOnlyInstance(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, sessions, p, rec := setupSablierWithMetrics(t)
		s.ActivityReconcileInterval = time.Hour
		s.SetGroups(map[string][]string{"default": {"plex", "other"}})
		sessions.EXPECT().Range(gomock.Any(), gomock.Any()).Return(nil)
		// Nothing is expected for "other": the mock fails the test on any call.
		sessions.EXPECT().Get(gomock.Any(), "plex").Return(readyInfo("plex"), nil)
		sessions.EXPECT().Put(gomock.Any(), readyInfo("plex"), 5*time.Minute).Return(nil)
		src := newFakeSource(map[string]uint64{"plex": 1000})
		expectListings(p, nil)
		events := startWatchActivity(t, s, p, src)
		// SessionDuration 0: the refresh uses DefaultSessionDuration (5m).
		started(events, activityInstance("plex", sablier.ActivityConfig{Ports: plexPorts, MinBytes: 1000}))

		nextActivityTick()

		assert.DeepEqual(t, sessionRequests(rec), []string{"session:activity/plex"})
	})
}

func TestWatchActivity_NeverShortensSession(t *testing.T) {
	tests := []struct {
		name   string
		groups []string
	}{
		{name: "group", groups: []string{"media"}},
		{name: "implicit default group", groups: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				s, sessions, p := setupSablier(t)
				s.ActivityReconcileInterval = time.Hour
				s.SetGroups(map[string][]string{"media": {"plex"}, "default": {"plex"}})
				// A 30m proxy session has 25m left.
				sessions.EXPECT().Range(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, f func(sablier.InstanceInfo, time.Time)) error {
					f(readyInfo("plex"), time.Now().Add(25*time.Minute))
					return nil
				})
				sessions.EXPECT().Get(gomock.Any(), "plex").Return(readyInfo("plex"), nil)
				sessions.EXPECT().Put(gomock.Any(), readyInfo("plex"), 25*time.Minute).Return(nil)
				src := newFakeSource(map[string]uint64{"plex": 1000})
				expectListings(p, nil)
				events := startWatchActivity(t, s, p, src)
				started(events, activityInstance("plex", sablier.ActivityConfig{
					Ports: plexPorts, SessionDuration: 5 * time.Minute, MinBytes: 1000, Groups: tt.groups,
				}))

				nextActivityTick()
			})
		})
	}
}

func TestWatchActivity_RangeErrorSkipsTick(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, sessions, p := setupSablier(t)
		s.ActivityReconcileInterval = time.Hour
		s.SetGroups(map[string][]string{"plex": {"plex"}})
		sessions.EXPECT().Range(gomock.Any(), gomock.Any()).Return(errors.New("store unavailable"))
		src := newFakeSource(map[string]uint64{"plex": 1000})
		expectListings(p, nil)
		events := startWatchActivity(t, s, p, src)
		started(events, activityInstance("plex", sablier.ActivityConfig{Ports: plexPorts, MinBytes: 1000, Groups: []string{"plex"}}))

		// No sessions.Get or Put is expected: the mock fails the test on any call.
		nextActivityTick()
	})
}

func TestWatchActivity_RecordsBytes(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, sessions, p, rec := setupSablierWithMetrics(t)
		s.ActivityReconcileInterval = time.Hour
		sessions.EXPECT().Range(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
		src := newFakeSource(map[string]uint64{"plex": 10})
		expectListings(p, nil)
		events := startWatchActivity(t, s, p, src)
		started(events, activityInstance("plex", sablier.ActivityConfig{Ports: plexPorts, MinBytes: 1000}))

		nextActivityTick()

		assert.Assert(t, slices.Contains(rec.snapshot(), "activity_bytes:plex/10"), "recorded: %v", rec.snapshot())
	})
}
