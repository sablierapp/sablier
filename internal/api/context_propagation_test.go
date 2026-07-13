package api

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sablierapp/sablier/pkg/sablier"
	"go.uber.org/mock/gomock"
	"gotest.tools/v3/assert"
)

type ctxSentinelKey struct{}

// TestStartDynamicPropagatesRequestContext pins that the dynamic handler hands
// the HTTP request's context to the session layer. Passing the *gin.Context
// itself instead of c.Request.Context() silently breaks two things (gin's
// ContextWithFallback is not enabled): Done() returns a nil channel, so
// client disconnects and server shutdown never cancel the session work, and
// Value() does not reach the request context, so the otelgin span is
// invisible downstream and every provider call becomes an orphan root span.
func TestStartDynamicPropagatesRequestContext(t *testing.T) {
	t.Run("names path", func(t *testing.T) {
		app, router, strategy, m := NewApiTest(t)
		StartDynamic(router, strategy)

		base := context.WithValue(context.Background(), ctxSentinelKey{}, "sentinel")
		reqCtx, cancel := context.WithCancel(base)
		cancel() // pre-cancelled: the handler must observe it

		var got context.Context
		m.EXPECT().RequestSession(gomock.Any(), []string{"test"}, gomock.Any()).
			DoAndReturn(func(ctx context.Context, _ []string, _ time.Duration) (*sablier.SessionState, error) {
				got = ctx
				return session(), nil
			})

		req := httptest.NewRequest("GET", "/api/strategies/dynamic?names=test", nil).WithContext(reqCtx)
		w := httptest.NewRecorder()
		app.ServeHTTP(w, req)

		assert.Assert(t, got != nil, "session layer was not called")
		assert.Equal(t, "sentinel", got.Value(ctxSentinelKey{}), "request context values must reach the session layer")
		assert.ErrorIs(t, got.Err(), context.Canceled, "request cancellation must reach the session layer")
	})

	t.Run("group path", func(t *testing.T) {
		app, router, strategy, m := NewApiTest(t)
		StartDynamic(router, strategy)

		base := context.WithValue(context.Background(), ctxSentinelKey{}, "sentinel")
		reqCtx, cancel := context.WithCancel(base)
		cancel()

		var got context.Context
		m.EXPECT().RequestSessionGroup(gomock.Any(), "test", gomock.Any()).
			DoAndReturn(func(ctx context.Context, _ string, _ time.Duration) (*sablier.SessionState, error) {
				got = ctx
				return session(), nil
			})

		req := httptest.NewRequest("GET", "/api/strategies/dynamic?group=test", nil).WithContext(reqCtx)
		w := httptest.NewRecorder()
		app.ServeHTTP(w, req)

		assert.Assert(t, got != nil, "session layer was not called")
		assert.Equal(t, "sentinel", got.Value(ctxSentinelKey{}), "request context values must reach the session layer")
		assert.ErrorIs(t, got.Err(), context.Canceled, "request cancellation must reach the session layer")
	})
}

// TestStartBlockingPropagatesRequestContext pins the same contract on the
// blocking handler, which already passed c.Request.Context() and must not
// regress to the raw gin context.
func TestStartBlockingPropagatesRequestContext(t *testing.T) {
	app, router, strategy, m := NewApiTest(t)
	StartBlocking(router, strategy)

	base := context.WithValue(context.Background(), ctxSentinelKey{}, "sentinel")
	reqCtx, cancel := context.WithCancel(base)
	cancel()

	var got context.Context
	m.EXPECT().RequestReadySession(gomock.Any(), []string{"test"}, gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, _ []string, _, _ time.Duration) (*sablier.SessionState, error) {
			got = ctx
			return session(), nil
		})

	req := httptest.NewRequest("GET", "/api/strategies/blocking?names=test", nil).WithContext(reqCtx)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	assert.Assert(t, got != nil, "session layer was not called")
	assert.Equal(t, "sentinel", got.Value(ctxSentinelKey{}), "request context values must reach the session layer")
	assert.ErrorIs(t, got.Err(), context.Canceled, "request cancellation must reach the session layer")
}
