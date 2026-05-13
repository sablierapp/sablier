package sablier_test

import (
	"testing"
	"time"

	"github.com/sablierapp/sablier/pkg/metrics"
	"github.com/sablierapp/sablier/pkg/sablier"
	"gotest.tools/v3/assert"
)

func TestSablierOnInstanceExpired_IgnoreUnlabeledEnabledStopsLabeledInstance(t *testing.T) {
	manager, _, provider, rec := setupSablierWithMetrics(t)
	manager.WithIgnoreUnlabeled(true)
	ctx := t.Context()

	provider.EXPECT().InstanceInspect(ctx, "nginx").Return(sablier.InstanceInfo{
		Name:    "nginx",
		Enabled: "true",
	}, nil)
	provider.EXPECT().InstanceStop(ctx, "nginx").Return(nil)

	manager.OnInstanceExpired(ctx)("nginx")

	assert.Assert(t, checkWithTimeout(50*time.Millisecond, 5*time.Second, func() bool {
		calls := rec.snapshot()
		return contains(calls, "stop:nginx/expired") &&
			contains(calls, "active-:nginx") &&
			contains(calls, "ready_discard:nginx")
	}), "expected expiration metrics")
}

func TestSablierOnInstanceExpired_IgnoreUnlabeledEnabledSkipsUnlabeledInstance(t *testing.T) {
	manager, _, provider, rec := setupSablierWithMetrics(t)
	manager.WithIgnoreUnlabeled(true)
	ctx := t.Context()
	inspected := make(chan struct{})

	provider.EXPECT().InstanceInspect(ctx, "nginx").DoAndReturn(func(_ interface{}, _ string) (sablier.InstanceInfo, error) {
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

func contains(calls []string, want string) bool {
	for _, c := range calls {
		if c == want {
			return true
		}
	}
	return false
}
