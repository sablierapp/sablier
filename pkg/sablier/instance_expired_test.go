package sablier_test

import (
	"errors"
	"testing"
	"time"

	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/sablier"
	"go.uber.org/mock/gomock"
	"gotest.tools/v3/assert"
)

func TestOnInstanceExpired_StopsAndRecordsMetrics(t *testing.T) {
	_, _, p, rec := setupSablierWithMetrics(t)
	ctx := t.Context()

	stopped := make(chan struct{}, 1)
	p.EXPECT().InstanceStop(gomock.Any(), "i1").DoAndReturn(func(_ any, _ string) error {
		stopped <- struct{}{}
		return nil
	})

	cb := sablier.OnInstanceExpired(ctx, p, rec, slogt.New(t))
	cb("i1")

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
	_, _, p, rec := setupSablierWithMetrics(t)
	ctx := t.Context()

	stopped := make(chan struct{}, 1)
	p.EXPECT().InstanceStop(gomock.Any(), "i2").DoAndReturn(func(_ any, _ string) error {
		stopped <- struct{}{}
		return errors.New("cannot stop")
	})

	cb := sablier.OnInstanceExpired(ctx, p, rec, slogt.New(t))
	cb("i2")

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

func containsCall(calls []string, want string) bool {
	for _, c := range calls {
		if c == want {
			return true
		}
	}
	return false
}
