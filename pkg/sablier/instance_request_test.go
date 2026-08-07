package sablier_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/sablierapp/sablier/pkg/sablier"
	"github.com/sablierapp/sablier/pkg/store"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.uber.org/mock/gomock"
	"gotest.tools/v3/assert"
)

// checkWithTimeout polls fn at the given interval until it returns true or the timeout expires.
func checkWithTimeout(interval, timeout time.Duration, fn func() bool) bool {
	deadline := time.After(timeout)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if fn() {
			return true
		}
		select {
		case <-deadline:
			return false
		case <-ticker.C:
		}
	}
}

func TestInstanceRequest_EmptyName(t *testing.T) {
	manager, _, _ := setupSablier(t)

	_, err := manager.InstanceRequest(t.Context(), "", time.Minute)
	assert.Error(t, err, "instance name cannot be empty")
}

func TestInstanceRequest_NewInstance_StartsAsync(t *testing.T) {
	manager, sessions, provider := setupSablier(t)
	ctx := t.Context()

	startCalled := make(chan struct{})

	stoppedInfo := sablier.InstanceInfo{
		Name: "nginx", CurrentReplicas: 0, DesiredReplicas: 1, Status: sablier.InstanceStatusStopped,
		Provider: "docker", Docker: &sablier.DockerContainerInfo{ID: "nginx", Image: "nginx:latest"},
	}
	notReady := stoppedInfo
	notReady.Status = sablier.InstanceStatusStarting

	sessions.EXPECT().Get(ctx, "nginx").Return(sablier.InstanceInfo{}, store.ErrKeyNotFound)
	provider.EXPECT().InstanceInspect(gomock.Any(), "nginx").Return(stoppedInfo, nil)

	provider.EXPECT().InstanceStart(gomock.Any(), "nginx").DoAndReturn(func(_ any, _ string) error {
		close(startCalled)
		return nil
	})

	sessions.EXPECT().Put(ctx, notReady, time.Minute).Return(nil)

	info, err := manager.InstanceRequest(ctx, "nginx", time.Minute)
	assert.NilError(t, err)
	assert.Equal(t, info.Status, sablier.InstanceStatus(sablier.InstanceStatusStarting))
	assert.Equal(t, info.Name, "nginx")

	select {
	case <-startCalled:
	case <-time.After(5 * time.Second):
		t.Fatal("InstanceStart was never called asynchronously")
	}
}

func TestRequestSession_NewUnlabeledRejectedWhenRejectUnlabeledRequestsEnabled(t *testing.T) {
	for _, enabled := range []string{"", "false"} {
		t.Run("enabled="+enabled, func(t *testing.T) {
			manager, sessions, provider := setupSablier(t)
			manager.WithRejectUnlabeledRequests(true)
			ctx := t.Context()

			stoppedInfo := sablier.InstanceInfo{
				Name:            "nginx",
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          sablier.InstanceStatusStopped,
				Enabled:         enabled,
			}

			sessions.EXPECT().Get(ctx, "nginx").Return(sablier.InstanceInfo{}, store.ErrKeyNotFound)
			provider.EXPECT().InstanceInspect(gomock.Any(), "nginx").Return(stoppedInfo, nil)

			session, err := manager.RequestSession(ctx, []string{"nginx"}, time.Minute)
			assert.NilError(t, err)

			notManaged, ok := errors.AsType[sablier.ErrInstanceNotManaged](session.Instances["nginx"].Error)
			assert.Assert(t, ok)
			assert.Equal(t, notManaged.Name, "nginx")
		})
	}
}

func TestRequestSession_NewLabeledInstanceStartsWhenRejectUnlabeledRequestsEnabled(t *testing.T) {
	manager, sessions, provider := setupSablier(t)
	manager.WithRejectUnlabeledRequests(true)
	ctx := t.Context()
	startCalled := make(chan struct{})

	stoppedInfo := sablier.InstanceInfo{
		Name:            "nginx",
		CurrentReplicas: 0,
		DesiredReplicas: 1,
		Status:          sablier.InstanceStatusStopped,
		Enabled:         "true",
	}
	notReady := stoppedInfo
	notReady.Status = sablier.InstanceStatusStarting

	sessions.EXPECT().Get(ctx, "nginx").Return(sablier.InstanceInfo{}, store.ErrKeyNotFound)
	provider.EXPECT().InstanceInspect(gomock.Any(), "nginx").Return(stoppedInfo, nil)
	provider.EXPECT().InstanceStart(gomock.Any(), "nginx").DoAndReturn(func(_ any, _ string) error {
		close(startCalled)
		return nil
	})
	sessions.EXPECT().Put(ctx, notReady, time.Minute).Return(nil)

	session, err := manager.RequestSession(ctx, []string{"nginx"}, time.Minute)
	assert.NilError(t, err)
	assert.Equal(t, session.Instances["nginx"].Instance.Status, sablier.InstanceStatus(sablier.InstanceStatusStarting))
	assert.NilError(t, session.Instances["nginx"].Error)

	select {
	case <-startCalled:
	case <-time.After(5 * time.Second):
		t.Fatal("InstanceStart was never called asynchronously")
	}
}

func TestInstanceRequest_NewInstance_ReturnsBeforeStartCompletes(t *testing.T) {
	manager, sessions, provider := setupSablier(t)
	ctx := t.Context()

	startBlocking := make(chan struct{})
	startCalled := make(chan struct{})

	stoppedInfo := sablier.InstanceInfo{
		Name: "nginx", CurrentReplicas: 0, DesiredReplicas: 1, Status: sablier.InstanceStatusStopped,
		Provider: "docker", Docker: &sablier.DockerContainerInfo{ID: "nginx", Image: "nginx:latest"},
	}
	notReady := stoppedInfo
	notReady.Status = sablier.InstanceStatusStarting

	sessions.EXPECT().Get(ctx, "nginx").Return(sablier.InstanceInfo{}, store.ErrKeyNotFound)
	provider.EXPECT().InstanceInspect(gomock.Any(), "nginx").Return(stoppedInfo, nil)

	provider.EXPECT().InstanceStart(gomock.Any(), "nginx").DoAndReturn(func(_ any, _ string) error {
		close(startCalled)
		<-startBlocking
		return nil
	})

	sessions.EXPECT().Put(ctx, notReady, time.Minute).Return(nil)

	info, err := manager.InstanceRequest(ctx, "nginx", time.Minute)
	assert.NilError(t, err)
	assert.Equal(t, info.Status, sablier.InstanceStatus(sablier.InstanceStatusStarting))

	select {
	case <-startCalled:
	case <-time.After(5 * time.Second):
		t.Fatal("InstanceStart was never called")
	}

	close(startBlocking)
}

func TestInstanceRequest_DuplicateCallsDoNotHammerProvider(t *testing.T) {
	manager, sessions, provider := setupSablier(t)
	ctx := t.Context()

	startBlocking := make(chan struct{})
	startCalled := make(chan struct{})

	stoppedInfo := sablier.InstanceInfo{
		Name: "nginx", CurrentReplicas: 0, DesiredReplicas: 1, Status: sablier.InstanceStatusStopped,
		Provider: "docker", Docker: &sablier.DockerContainerInfo{ID: "nginx", Image: "nginx:latest"},
	}
	notReady := stoppedInfo
	notReady.Status = sablier.InstanceStatusStarting

	// First call: store miss -> triggers requestStart
	// Second call: store returns the not-ready state written by Put -> hits status != ready path
	gomock.InOrder(
		sessions.EXPECT().Get(ctx, "nginx").Return(sablier.InstanceInfo{}, store.ErrKeyNotFound),
		sessions.EXPECT().Get(ctx, "nginx").Return(notReady, nil),
	)

	// InstanceInspect: exactly once during requestStart (second call must not trigger another)
	provider.EXPECT().InstanceInspect(gomock.Any(), "nginx").Return(stoppedInfo, nil).Times(1)

	// InstanceStart: exactly once (second call must not trigger another)
	provider.EXPECT().InstanceStart(gomock.Any(), "nginx").DoAndReturn(func(_ any, _ string) error {
		close(startCalled)
		<-startBlocking
		return nil
	}).Times(1)

	// Second call: start is still pending -> skips InstanceInspect, returns not-ready
	sessions.EXPECT().Put(ctx, notReady, time.Minute).Return(nil).Times(2)

	// First call — starts the goroutine
	info1, err := manager.InstanceRequest(ctx, "nginx", time.Minute)
	assert.NilError(t, err)
	assert.Equal(t, info1.Status, sablier.InstanceStatus(sablier.InstanceStatusStarting))

	// Wait for the goroutine to enter InstanceStart
	select {
	case <-startCalled:
	case <-time.After(5 * time.Second):
		t.Fatal("InstanceStart was never called")
	}

	// Second call — start still pending, skips inspect, no duplicate InstanceStart
	info2, err := manager.InstanceRequest(ctx, "nginx", time.Minute)
	assert.NilError(t, err)
	assert.Equal(t, info2.Status, sablier.InstanceStatus(sablier.InstanceStatusStarting))

	close(startBlocking)
}

// TestInstanceRequest_SecondCallJoinsInFlightPendingStart verifies that when a
// second InstanceRequest arrives for the same instance while the first request's
// async InstanceStart goroutine is still running, the second request takes the
// "default" branch in requestStart's first critical section:
//   - InstanceInspect is NOT called again
//   - InstanceStart is NOT called again
//   - The second call returns the cached "starting" InstanceInfo immediately
//   - A "sablier.instance.join_pending_start" OTel event is recorded on the
//     second request's span so the two requests can be correlated in the backend.
func TestInstanceRequest_SecondCallJoinsInFlightPendingStart(t *testing.T) {
	// Wire up an in-memory OTel exporter so we can assert the join span event.
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		_ = tp.Shutdown(context.Background())
		otel.SetTracerProvider(otel.GetTracerProvider())
	})

	manager, sessions, provider := setupSablier(t)
	tracer := tp.Tracer("test")

	startBlocking := make(chan struct{})
	startCalled := make(chan struct{})

	stoppedInfo := sablier.InstanceInfo{
		Name: "nginx", CurrentReplicas: 0, DesiredReplicas: 1,
		Status: sablier.InstanceStatusStopped,
	}
	notReady := stoppedInfo
	notReady.Status = sablier.InstanceStatusStarting

	// Both calls return ErrKeyNotFound so requestStart is entered twice.
	// The second must coalesce onto the in-flight goroutine without retrying.
	sessions.EXPECT().Get(gomock.Any(), "nginx").Return(sablier.InstanceInfo{}, store.ErrKeyNotFound).Times(2)
	provider.EXPECT().InstanceInspect(gomock.Any(), "nginx").Return(stoppedInfo, nil).Times(1)
	provider.EXPECT().InstanceStart(gomock.Any(), "nginx").DoAndReturn(func(_ any, _ string) error {
		close(startCalled)
		<-startBlocking
		return nil
	}).Times(1)
	sessions.EXPECT().Put(gomock.Any(), notReady, time.Minute).Return(nil).Times(2)

	// First request — creates the pending entry and dispatches the goroutine.
	firstCtx, firstSpan := tracer.Start(t.Context(), "first-request")
	info1, err := manager.InstanceRequest(firstCtx, "nginx", time.Minute)
	firstSpan.End()
	assert.NilError(t, err)
	assert.Equal(t, info1.Status, sablier.InstanceStatus(sablier.InstanceStatusStarting))

	// Ensure the goroutine has entered InstanceStart before the second request.
	select {
	case <-startCalled:
	case <-time.After(5 * time.Second):
		t.Fatal("InstanceStart goroutine never started")
	}

	// Second request — goroutine still blocked: must take the "default" branch in
	// requestStart's first critical section, returning the cached info without
	// calling InstanceInspect or InstanceStart again.
	secondCtx, secondSpan := tracer.Start(t.Context(), "second-request")
	info2, err := manager.InstanceRequest(secondCtx, "nginx", time.Minute)
	secondSpan.End()
	assert.NilError(t, err)
	assert.Equal(t, info2.Status, sablier.InstanceStatus(sablier.InstanceStatusStarting))
	assert.Equal(t, info2.Name, info1.Name)

	// Verify the join event was recorded on the second request's span, and that
	// it references the first request's trace/span IDs.
	var foundEvent bool
	for _, s := range exporter.GetSpans() {
		if s.Name != "second-request" {
			continue
		}
		for _, e := range s.Events {
			if e.Name == "sablier.instance.join_pending_start" {
				foundEvent = true
				// The event must reference the first request's trace context.
				var gotTraceID, gotSpanID string
				for _, attr := range e.Attributes {
					switch string(attr.Key) {
					case "pending_trace_id":
						gotTraceID = attr.Value.AsString()
					case "pending_span_id":
						gotSpanID = attr.Value.AsString()
					}
				}
				assert.Equal(t, gotTraceID, firstSpan.SpanContext().TraceID().String(),
					"join event must reference the first request's trace ID")
				assert.Equal(t, gotSpanID, firstSpan.SpanContext().SpanID().String(),
					"join event must reference the first request's span ID")
			}
		}
	}
	assert.Assert(t, foundEvent, "expected sablier.instance.join_pending_start event on second-request span")

	close(startBlocking)
}

func TestInstanceRequest_AsyncErrorSurfacedOnNotReadyPath(t *testing.T) {
	manager, sessions, provider := setupSablier(t)
	ctx := t.Context()

	stoppedInfo := sablier.InstanceInfo{
		Name: "nginx", CurrentReplicas: 0, DesiredReplicas: 1, Status: sablier.InstanceStatusStopped,
		Provider: "docker", Docker: &sablier.DockerContainerInfo{ID: "nginx", Image: "nginx:latest"},
	}
	notReady := stoppedInfo
	notReady.Status = sablier.InstanceStatusStarting

	// First call: store miss -> requestStart
	sessions.EXPECT().Get(ctx, "nginx").Return(sablier.InstanceInfo{}, store.ErrKeyNotFound)
	provider.EXPECT().InstanceInspect(gomock.Any(), "nginx").Return(stoppedInfo, nil)
	sessions.EXPECT().Put(ctx, notReady, time.Minute).Return(nil)

	// Goroutine fails immediately
	provider.EXPECT().InstanceStart(gomock.Any(), "nginx").Return(errors.New("connection refused"))

	info, err := manager.InstanceRequest(ctx, "nginx", time.Minute)
	assert.NilError(t, err)
	assert.Equal(t, info.Status, sablier.InstanceStatus(sablier.InstanceStatusStarting))

	// Subsequent calls: store returns the stored not-ready state (realistic behavior).
	// While goroutine is still running, polling returns not-ready with Put.
	// Once goroutine finishes, consumePendingError surfaces the error (no Put in that case).
	sessions.EXPECT().Get(ctx, "nginx").Return(notReady, nil).AnyTimes()
	sessions.EXPECT().Put(ctx, notReady, time.Minute).Return(nil).AnyTimes()

	assert.Assert(t, checkWithTimeout(100*time.Millisecond, 5*time.Second, func() bool {
		_, err = manager.InstanceRequest(ctx, "nginx", time.Minute)
		return err != nil
	}), "expected async error to be surfaced on the not-ready path")
	assert.ErrorContains(t, err, "instance start failed: connection refused")
}

func TestInstanceRequest_RetryAfterErrorConsumed(t *testing.T) {
	manager, sessions, provider := setupSablier(t)
	ctx := t.Context()

	stoppedInfo := sablier.InstanceInfo{
		Name: "nginx", CurrentReplicas: 0, DesiredReplicas: 1, Status: sablier.InstanceStatusStopped,
		Provider: "docker", Docker: &sablier.DockerContainerInfo{ID: "nginx", Image: "nginx:latest"},
	}
	notReady := stoppedInfo
	notReady.Status = sablier.InstanceStatusStarting
	secondDone := make(chan struct{})

	// All Get/Put/Inspect calls use AnyTimes since polling may hit them multiple times
	sessions.EXPECT().Get(ctx, "nginx").Return(sablier.InstanceInfo{}, store.ErrKeyNotFound).AnyTimes()
	sessions.EXPECT().Put(ctx, notReady, time.Minute).Return(nil).AnyTimes()
	provider.EXPECT().InstanceInspect(gomock.Any(), "nginx").Return(stoppedInfo, nil).AnyTimes()

	gomock.InOrder(
		// First attempt — fails immediately
		provider.EXPECT().InstanceStart(gomock.Any(), "nginx").Return(errors.New("connection refused")),
		// Retry — succeeds
		provider.EXPECT().InstanceStart(gomock.Any(), "nginx").DoAndReturn(func(_ any, _ string) error {
			close(secondDone)
			return nil
		}),
	)

	// 1st call: dispatches goroutine (fails)
	_, err := manager.InstanceRequest(ctx, "nginx", time.Minute)
	assert.NilError(t, err)

	// 2nd call: poll until the error is consumable
	assert.Assert(t, checkWithTimeout(100*time.Millisecond, 5*time.Second, func() bool {
		_, err = manager.InstanceRequest(ctx, "nginx", time.Minute)
		return err != nil
	}), "expected error to be surfaced")
	assert.ErrorContains(t, err, "instance start failed: connection refused")

	// 3rd call: entry cleared, store miss again -> requestStart retries
	info, err := manager.InstanceRequest(ctx, "nginx", time.Minute)
	assert.NilError(t, err)
	assert.Equal(t, info.Status, sablier.InstanceStatus(sablier.InstanceStatusStarting))

	select {
	case <-secondDone:
	case <-time.After(5 * time.Second):
		t.Fatal("Retry goroutine was never started")
	}
}

func TestInstanceRequest_SuccessfulStartCleansUpPendingEntry(t *testing.T) {
	manager, sessions, provider := setupSablier(t)
	ctx := t.Context()

	stoppedInfo := sablier.InstanceInfo{
		Name: "nginx", CurrentReplicas: 0, DesiredReplicas: 1, Status: sablier.InstanceStatusStopped,
		Provider: "docker", Docker: &sablier.DockerContainerInfo{ID: "nginx", Image: "nginx:latest"},
	}
	notReady := stoppedInfo
	notReady.Status = sablier.InstanceStatusStarting
	ready := sablier.InstanceInfo{Name: "nginx", CurrentReplicas: 1, DesiredReplicas: 1, Status: sablier.InstanceStatusReady}

	// 1st call: store miss -> requestStart (goroutine succeeds)
	sessions.EXPECT().Get(ctx, "nginx").Return(sablier.InstanceInfo{}, store.ErrKeyNotFound)
	sessions.EXPECT().Put(ctx, notReady, time.Minute).Return(nil)

	startDone := make(chan struct{})
	gomock.InOrder(
		provider.EXPECT().InstanceInspect(gomock.Any(), "nginx").Return(stoppedInfo, nil), // pre-start inspect
		provider.EXPECT().InstanceInspect(gomock.Any(), "nginx").Return(ready, nil),       // post-start inspect in not-ready path
	)
	provider.EXPECT().InstanceStart(gomock.Any(), "nginx").DoAndReturn(func(_ any, _ string) error {
		close(startDone)
		return nil
	})

	info, err := manager.InstanceRequest(ctx, "nginx", time.Minute)
	assert.NilError(t, err)
	assert.Equal(t, info.Status, sablier.InstanceStatus(sablier.InstanceStatusStarting))

	// Wait for goroutine to finish and self-clean
	select {
	case <-startDone:
	case <-time.After(5 * time.Second):
		t.Fatal("InstanceStart goroutine never completed")
	}
	// Small settle time for the goroutine to acquire the lock and clean up
	time.Sleep(50 * time.Millisecond)

	// 2nd call: store returns not-ready, no pending entry exists, goes straight to inspect
	sessions.EXPECT().Get(ctx, "nginx").Return(notReady, nil)
	sessions.EXPECT().Put(ctx, readyAtMatcher{}, time.Minute).Return(nil)

	info, err = manager.InstanceRequest(ctx, "nginx", time.Minute)
	assert.NilError(t, err)
	assert.Equal(t, info.Status, sablier.InstanceStatus(sablier.InstanceStatusReady))
}

func TestInstanceRequest_StartTimeoutSurfacesError(t *testing.T) {
	manager, sessions, provider := setupSablier(t)
	manager.InstanceStartTimeout = 100 * time.Millisecond
	ctx := t.Context()

	stoppedInfo := sablier.InstanceInfo{
		Name: "nginx", CurrentReplicas: 0, DesiredReplicas: 1, Status: sablier.InstanceStatusStopped,
		Provider: "docker", Docker: &sablier.DockerContainerInfo{ID: "nginx", Image: "nginx:latest"},
	}
	notReady := stoppedInfo
	notReady.Status = sablier.InstanceStatusStarting

	sessions.EXPECT().Get(ctx, "nginx").Return(sablier.InstanceInfo{}, store.ErrKeyNotFound)
	provider.EXPECT().InstanceInspect(gomock.Any(), "nginx").Return(stoppedInfo, nil)
	sessions.EXPECT().Put(ctx, notReady, time.Minute).Return(nil)

	// InstanceStart blocks until context is cancelled
	provider.EXPECT().InstanceStart(gomock.Any(), "nginx").DoAndReturn(func(startCtx any, _ string) error {
		<-startCtx.(interface{ Done() <-chan struct{} }).Done()
		return startCtx.(interface{ Err() error }).Err()
	})

	info, err := manager.InstanceRequest(ctx, "nginx", time.Minute)
	assert.NilError(t, err)
	assert.Equal(t, info.Status, sablier.InstanceStatus(sablier.InstanceStatusStarting))

	// Subsequent calls: store returns not-ready; polling may get not-ready while
	// the goroutine is still in progress, or the timeout error once it completes.
	sessions.EXPECT().Get(ctx, "nginx").Return(notReady, nil).AnyTimes()
	sessions.EXPECT().Put(ctx, notReady, time.Minute).Return(nil).AnyTimes()

	assert.Assert(t, checkWithTimeout(50*time.Millisecond, 5*time.Second, func() bool {
		_, err = manager.InstanceRequest(ctx, "nginx", time.Minute)
		return err != nil
	}), "expected timeout error to be surfaced")
	assert.ErrorContains(t, err, "instance start failed")
}

func TestInstanceRequest_ExistingNotReady_InspectsProvider(t *testing.T) {
	manager, sessions, provider := setupSablier(t)
	ctx := t.Context()

	sessions.EXPECT().Get(ctx, "nginx").Return(sablier.InstanceInfo{
		Name:   "nginx",
		Status: sablier.InstanceStatusStarting,
	}, nil)

	provider.EXPECT().InstanceInspect(gomock.Any(), "nginx").Return(sablier.InstanceInfo{
		Name:            "nginx",
		CurrentReplicas: 1,
		DesiredReplicas: 1,
		Status:          sablier.InstanceStatusReady,
	}, nil)

	sessions.EXPECT().Put(ctx, readyAtMatcher{}, time.Minute).Return(nil)

	info, err := manager.InstanceRequest(ctx, "nginx", time.Minute)
	assert.NilError(t, err)
	assert.Equal(t, info.Status, sablier.InstanceStatus(sablier.InstanceStatusReady))
}

func TestInstanceRequest_ExistingReady_SkipsInspect(t *testing.T) {
	manager, sessions, _ := setupSablier(t)
	ctx := t.Context()

	sessions.EXPECT().Get(ctx, "nginx").Return(sablier.InstanceInfo{
		Name:            "nginx",
		CurrentReplicas: 1,
		DesiredReplicas: 1,
		Status:          sablier.InstanceStatusReady,
	}, nil)

	sessions.EXPECT().Put(ctx, sablier.InstanceInfo{
		Name:            "nginx",
		CurrentReplicas: 1,
		DesiredReplicas: 1,
		Status:          sablier.InstanceStatusReady,
	}, time.Minute).Return(nil)

	info, err := manager.InstanceRequest(ctx, "nginx", time.Minute)
	assert.NilError(t, err)
	assert.Equal(t, info.Status, sablier.InstanceStatus(sablier.InstanceStatusReady))
}

func TestInstanceRequest_StoreGetError(t *testing.T) {
	manager, sessions, _ := setupSablier(t)
	ctx := t.Context()

	sessions.EXPECT().Get(ctx, "nginx").Return(sablier.InstanceInfo{}, errors.New("connection refused"))

	_, err := manager.InstanceRequest(ctx, "nginx", time.Minute)
	assert.ErrorContains(t, err, "cannot retrieve instance from store")
}

func TestInstanceRequest_NewInstance_RecordsStartMetrics_Success(t *testing.T) {
	manager, sessions, provider, rec := setupSablierWithMetrics(t)
	ctx := t.Context()

	startDone := make(chan struct{})

	stoppedInfo := sablier.InstanceInfo{
		Name: "nginx", CurrentReplicas: 0, DesiredReplicas: 1, Status: sablier.InstanceStatusStopped,
		Provider: "docker", Docker: &sablier.DockerContainerInfo{ID: "nginx", Image: "nginx:latest"},
	}
	notReady := stoppedInfo
	notReady.Status = sablier.InstanceStatusStarting

	sessions.EXPECT().Get(ctx, "nginx").Return(sablier.InstanceInfo{}, store.ErrKeyNotFound)
	provider.EXPECT().InstanceInspect(gomock.Any(), "nginx").Return(stoppedInfo, nil)
	sessions.EXPECT().Put(ctx, notReady, time.Minute).Return(nil)

	provider.EXPECT().InstanceStart(gomock.Any(), "nginx").DoAndReturn(func(_ any, _ string) error {
		close(startDone)
		return nil
	})

	_, err := manager.InstanceRequest(ctx, "nginx", time.Minute)
	assert.NilError(t, err)

	select {
	case <-startDone:
	case <-time.After(5 * time.Second):
		t.Fatal("InstanceStart goroutine never completed")
	}
	// Settle for the goroutine to record the end metric.
	assert.Assert(t, checkWithTimeout(50*time.Millisecond, 5*time.Second, func() bool {
		return slices.Contains(rec.snapshot(), "start_end:nginx")
	}), "expected start_end metric")

	calls := rec.snapshot()
	assertContains(t, calls, "ready_begin:nginx")
	assertContains(t, calls, "active+:nginx")
}

func TestInstanceRequest_NewInstance_RecordsStartFailure(t *testing.T) {
	manager, sessions, provider, rec := setupSablierWithMetrics(t)
	ctx := t.Context()

	stoppedInfo := sablier.InstanceInfo{
		Name: "nginx", CurrentReplicas: 0, DesiredReplicas: 1, Status: sablier.InstanceStatusStopped,
		Provider: "docker", Docker: &sablier.DockerContainerInfo{ID: "nginx", Image: "nginx:latest"},
	}
	notReady := stoppedInfo
	notReady.Status = sablier.InstanceStatusStarting

	sessions.EXPECT().Get(ctx, "nginx").Return(sablier.InstanceInfo{}, store.ErrKeyNotFound)
	provider.EXPECT().InstanceInspect(gomock.Any(), "nginx").Return(stoppedInfo, nil)
	sessions.EXPECT().Put(ctx, notReady, time.Minute).Return(nil)
	provider.EXPECT().InstanceStart(gomock.Any(), "nginx").Return(errors.New("boom"))

	_, err := manager.InstanceRequest(ctx, "nginx", time.Minute)
	assert.NilError(t, err) // first call returns not-ready, error surfaces on next

	assert.Assert(t, checkWithTimeout(50*time.Millisecond, 5*time.Second, func() bool {
		return slices.Contains(rec.snapshot(), "start_fail:nginx")
	}), "expected start_fail metric")

	calls := rec.snapshot()
	for _, c := range calls {
		if c == "start_end:nginx" {
			t.Errorf("did not expect start_end on failure, got: %v", calls)
		}
	}
}

func TestInstanceRequest_ReadyTransition_RecordsReadyEnd(t *testing.T) {
	manager, sessions, provider, rec := setupSablierWithMetrics(t)
	ctx := t.Context()

	notReady := sablier.InstanceInfo{
		Name: "nginx", CurrentReplicas: 0, DesiredReplicas: 1, Status: sablier.InstanceStatusStarting,
	}
	ready := sablier.InstanceInfo{
		Name: "nginx", CurrentReplicas: 1, DesiredReplicas: 1, Status: sablier.InstanceStatusReady,
	}

	sessions.EXPECT().Get(ctx, "nginx").Return(notReady, nil)
	provider.EXPECT().InstanceInspect(gomock.Any(), "nginx").Return(ready, nil)
	sessions.EXPECT().Put(ctx, readyAtMatcher{}, time.Minute).Return(nil)

	// Pre-seed the ready-wait state by simulating a previous Begin.
	rec.RecordReadyWaitBegin("nginx")

	_, err := manager.InstanceRequest(ctx, "nginx", time.Minute)
	assert.NilError(t, err)

	calls := rec.snapshot()
	assertContains(t, calls, "ready_end:nginx")
}

func assertContains(t *testing.T, calls []string, want string) {
	t.Helper()
	if slices.Contains(calls, want) {
		return
	}
	t.Errorf("expected %q in calls, got: %v", want, calls)
}

// readyAtMatcher is a gomock matcher that accepts any InstanceInfo whose
// Status is Ready and ReadyAt is non-nil. This is needed because ReadyAt is
// stamped with time.Now() inside InstanceRequest and cannot be predicted.
type readyAtMatcher struct{}

func (readyAtMatcher) Matches(x any) bool {
	info, ok := x.(sablier.InstanceInfo)
	return ok && info.Status == sablier.InstanceStatusReady && info.ReadyAt != nil
}
func (readyAtMatcher) String() string {
	return "InstanceInfo{Status: ready, ReadyAt: non-nil}"
}

func TestInstanceRequest_ReadyAfter_StampsReadyAt(t *testing.T) {
	manager, sessions, provider := setupSablier(t)
	ctx := t.Context()

	startingInfo := sablier.InstanceInfo{
		Name: "nginx", Status: sablier.InstanceStatusStarting,
	}
	// Provider reports ready and carries the ReadyAfter label value.
	readyFromProvider := sablier.InstanceInfo{
		Name:       "nginx",
		Status:     sablier.InstanceStatusReady,
		ReadyAfter: 100 * time.Millisecond,
	}

	// Store has a Starting entry — triggers the inspect path.
	sessions.EXPECT().Get(ctx, "nginx").Return(startingInfo, nil)
	provider.EXPECT().InstanceInspect(gomock.Any(), "nginx").Return(readyFromProvider, nil)
	// Put must be called with a Ready state that has ReadyAt stamped.
	sessions.EXPECT().Put(ctx, readyAtMatcher{}, time.Minute).Return(nil)

	info, err := manager.InstanceRequest(ctx, "nginx", time.Minute)
	assert.NilError(t, err)
	assert.Equal(t, info.Status, sablier.InstanceStatusReady)
	assert.Assert(t, info.ReadyAt != nil, "ReadyAt should be stamped on first Ready transition")
	assert.Equal(t, info.ReadyAfter, 100*time.Millisecond)
	// Within the grace period — IsReady() should still return false.
	assert.Assert(t, !info.IsReady(), "IsReady() should be false within ReadyAfter grace period")
}

func TestInstanceRequest_ReadyAfter_GracePeriodRespectedByPolling(t *testing.T) {
	manager, sessions, provider := setupSablier(t)
	manager.BlockingRefreshFrequency = 10 * time.Millisecond
	ctx := t.Context()

	startingInfo := sablier.InstanceInfo{Name: "nginx", Status: sablier.InstanceStatusStarting}
	readyFromProvider := sablier.InstanceInfo{
		Name:       "nginx",
		Status:     sablier.InstanceStatusReady,
		ReadyAfter: 150 * time.Millisecond,
	}

	// First RequestSession call: store returns Starting → inspect → stamp ReadyAt.
	sessions.EXPECT().Get(gomock.Any(), "nginx").Return(startingInfo, nil).Times(1)
	provider.EXPECT().InstanceInspect(gomock.Any(), "nginx").Return(readyFromProvider, nil).Times(1)
	sessions.EXPECT().Put(gomock.Any(), readyAtMatcher{}, time.Minute).Return(nil).Times(1)

	// Subsequent polling calls: store returns the stamped-Ready state.
	// ReadyAt is in the past by a growing amount; sessions.Put is called each tick.
	sessions.EXPECT().Get(gomock.Any(), "nginx").DoAndReturn(func(_ any, _ string) (sablier.InstanceInfo, error) {
		// Simulate the store returning the previously stored Ready+ReadyAt state.
		past := time.Now().Add(-200 * time.Millisecond) // 200ms > 150ms ReadyAfter
		return sablier.InstanceInfo{
			Name:       "nginx",
			Status:     sablier.InstanceStatusReady,
			ReadyAfter: 150 * time.Millisecond,
			ReadyAt:    &past,
		}, nil
	}).AnyTimes()
	sessions.EXPECT().Put(gomock.Any(), gomock.Any(), time.Minute).Return(nil).AnyTimes()

	session, err := manager.RequestReadySession(ctx, []string{"nginx"}, time.Minute, 5*time.Second)
	assert.NilError(t, err)
	assert.Assert(t, session.IsReady())
}
