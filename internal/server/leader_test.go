package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/internal/api/apitest"
	"github.com/sablierapp/sablier/pkg/config"
	"go.uber.org/mock/gomock"
	"gotest.tools/v3/assert"
)

func get(r http.Handler, target string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", target, nil))
	return w
}

func TestLeaderElection_FollowerRejectsSessionRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	// A follower must never act on workloads: no session request may reach Sablier.
	m := apitest.NewMockSablier(ctrl)

	var leading atomic.Bool
	strategy := testStrategy(t)
	strategy.Sablier = m
	strategy.Leading = leading.Load
	r := setupRouter(context.Background(), slogt.New(t), config.Server{BasePath: "/"}, config.Tracing{ServiceName: "sablier"}, strategy)

	// Followers stay healthy, so Deployment rollouts are unaffected.
	assert.Equal(t, get(r, "/health").Code, http.StatusOK)
	assert.Equal(t, get(r, "/api/themes").Code, http.StatusOK)
	for _, target := range []string{
		"/api/strategies/dynamic?names=test",
		"/api/strategies/blocking?names=test",
		"/api/strategies/poke?names=test",
		"/api/events",
	} {
		w := get(r, target)
		assert.Equal(t, w.Code, http.StatusServiceUnavailable, target)
		assert.Equal(t, w.Header().Get("Content-Type"), "application/problem+json", target)
	}

	leading.Store(true)
	m.EXPECT().RequestReadySession(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(readySession(), nil)

	assert.Equal(t, get(r, "/api/strategies/blocking?names=test").Code, http.StatusOK)
}
