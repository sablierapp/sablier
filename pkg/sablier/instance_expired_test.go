package sablier_test

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/sablierapp/sablier/pkg/metrics"
	"github.com/sablierapp/sablier/pkg/sablier"
	"go.uber.org/mock/gomock"
	"gotest.tools/v3/assert"
)

func TestOnInstanceExpired_StopsAndRecordsMetrics(t *testing.T) {
	manager, _, p, rec := setupSablierWithMetrics(t)
	ctx := t.Context()

	p.EXPECT().InstanceInspect(gomock.Any(), "i1").Return(sablier.InstanceInfo{Name: "i1"}, nil)
	stopped := make(chan struct{}, 1)
	p.EXPECT().InstanceStop(gomock.Any(), "i1").DoAndReturn(func(_ any, _ string) error {
		stopped <- struct{}{}
		return nil
	})

	manager.OnInstanceExpired(ctx)("i1")

	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("instance stop was not invoked")
	}

	// Let recorder calls complete in the callback goroutine.
	time.Sleep(20 * time.Millisecond)
	calls := rec.snapshot()
	assert.Assert(t, containsCall(calls, "stop:i1/expired"))
	assert.Assert(t, containsCall(calls, "active-:i1"))
	assert.Assert(t, containsCall(calls, "ready_discard:i1"))
}

func TestOnInstanceExpired_ProviderStopErrorStillRecordsMetrics(t *testing.T) {
	manager, _, p, rec := setupSablierWithMetrics(t)
	ctx := t.Context()

	p.EXPECT().InstanceInspect(gomock.Any(), "i2").Return(sablier.InstanceInfo{Name: "i2"}, nil)
	stopped := make(chan struct{}, 1)
	p.EXPECT().InstanceStop(gomock.Any(), "i2").DoAndReturn(func(_ any, _ string) error {
		stopped <- struct{}{}
		return errors.New("cannot stop")
	})

	manager.OnInstanceExpired(ctx)("i2")

	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("instance stop was not invoked")
	}

	time.Sleep(20 * time.Millisecond)
	calls := rec.snapshot()
	assert.Assert(t, containsCall(calls, "stop:i2/expired"))
	assert.Assert(t, containsCall(calls, "active-:i2"))
	assert.Assert(t, containsCall(calls, "ready_discard:i2"))
}

func TestSablierOnInstanceExpired_VerifyEnabledOnExpirationStopsLabeledInstance(t *testing.T) {
	manager, _, provider, rec := setupSablierWithMetrics(t)
	manager.WithVerifyEnabledOnExpiration(true)
	ctx := t.Context()

	provider.EXPECT().InstanceInspect(ctx, "nginx").Return(sablier.InstanceInfo{
		Name:    "nginx",
		Enabled: "true",
	}, nil)
	provider.EXPECT().InstanceStop(ctx, "nginx").Return(nil)

	manager.OnInstanceExpired(ctx)("nginx")

	assert.Assert(t, checkWithTimeout(50*time.Millisecond, 5*time.Second, func() bool {
		calls := rec.snapshot()
		return containsCall(calls, "stop:nginx/expired") &&
			containsCall(calls, "active-:nginx") &&
			containsCall(calls, "ready_discard:nginx")
	}), "expected expiration metrics")
}

func TestSablierOnInstanceExpired_VerifyEnabledOnExpirationSkipsUnlabeledInstance(t *testing.T) {
	manager, _, provider, rec := setupSablierWithMetrics(t)
	manager.WithVerifyEnabledOnExpiration(true)
	ctx := t.Context()
	inspected := make(chan struct{})

	provider.EXPECT().InstanceInspect(ctx, "nginx").DoAndReturn(func(_ any, _ string) (sablier.InstanceInfo, error) {
		close(inspected)
		return sablier.InstanceInfo{Name: "nginx"}, nil
	})

	manager.OnInstanceExpired(ctx)("nginx")

	select {
	case <-inspected:
	case <-time.After(5 * time.Second):
		t.Fatal("InstanceInspect was never called")
	}
	time.Sleep(50 * time.Millisecond)

	assertNoExpirationMetrics(t, rec)
}

func assertNoExpirationMetrics(t *testing.T, rec metrics.Recorder) {
	t.Helper()
	fake := rec.(*fakeRecorder)
	for _, c := range fake.snapshot() {
		if c == "stop:nginx/expired" || c == "active-:nginx" || c == "ready_discard:nginx" {
			t.Fatalf("did not expect expiration metric %q, got: %v", c, fake.snapshot())
		}
	}
}

func containsCall(calls []string, want string) bool {
	return slices.Contains(calls, want)
}

// TestSablierOnInstanceExpired_Delegated emits a deactivate intent and does not
// call the provider's InstanceStop (the absence of an EXPECT makes gomock fail
// if it is called).
func TestSablierOnInstanceExpired_Delegated(t *testing.T) {
	manager, _, provider, rec := setupSablierWithMetrics(t)
	ctx := t.Context()

	provider.EXPECT().InstanceInspect(gomock.Any(), "app").Return(sablier.InstanceInfo{
		Name: "app", Enabled: "true", Config: &sablier.InstanceConfig{Enabled: true, DelegateScaling: true},
	}, nil)

	stream := manager.IntentEvents(ctx)
	manager.OnInstanceExpired(ctx)("app")

	select {
	case ev := <-stream.Events:
		assert.Equal(t, string(ev.Type), "deactivate")
		assert.Equal(t, ev.Info.Name, "app")
	case <-time.After(2 * time.Second):
		t.Fatal("expected a deactivate intent event")
	}

	assert.Assert(t, checkWithTimeout(20*time.Millisecond, 2*time.Second, func() bool {
		calls := rec.snapshot()
		return containsCall(calls, "stop:app/expired") &&
			containsCall(calls, "active-:app") &&
			containsCall(calls, "ready_discard:app")
	}), "expected expiration metrics")
}
