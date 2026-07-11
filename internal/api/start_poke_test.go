package api

import (
	"errors"
	"net/http"
	"testing"

	"github.com/sablierapp/sablier/pkg/sablier"
	"github.com/tniswong/go.rfcx/rfc7807"
	"go.uber.org/mock/gomock"
	"gotest.tools/v3/assert"
)

func TestStartPoke(t *testing.T) {
	t.Run("StartPokeInvalidBind", func(t *testing.T) {
		app, router, strategy, _ := NewApiTest(t)
		StartPoke(router, strategy)
		r := PerformRequest(app, "GET", "/api/strategies/poke?session_duration=invalid")
		assert.Equal(t, http.StatusBadRequest, r.Code)
		assert.Equal(t, rfc7807.JSONMediaType, r.Header().Get("Content-Type"))
	})
	t.Run("StartPokeWithoutNamesOrGroup", func(t *testing.T) {
		app, router, strategy, _ := NewApiTest(t)
		StartPoke(router, strategy)
		r := PerformRequest(app, "GET", "/api/strategies/poke")
		assert.Equal(t, http.StatusBadRequest, r.Code)
		assert.Equal(t, rfc7807.JSONMediaType, r.Header().Get("Content-Type"))
	})
	t.Run("StartPokeWithNamesAndGroup", func(t *testing.T) {
		app, router, strategy, _ := NewApiTest(t)
		StartPoke(router, strategy)
		r := PerformRequest(app, "GET", "/api/strategies/poke?names=test&group=test")
		assert.Equal(t, http.StatusBadRequest, r.Code)
		assert.Equal(t, rfc7807.JSONMediaType, r.Header().Get("Content-Type"))
	})
	t.Run("StartPokeByNames", func(t *testing.T) {
		app, router, strategy, m := NewApiTest(t)
		StartPoke(router, strategy)
		m.EXPECT().RequestSession(gomock.Any(), []string{"test"}, gomock.Any()).Return(&sablier.SessionState{}, nil)
		r := PerformRequest(app, "GET", "/api/strategies/poke?names=test")
		assert.Equal(t, http.StatusOK, r.Code)
		assert.Equal(t, SablierStatusReady, r.Header().Get(SablierStatusHeader))
	})
	t.Run("StartPokeByGroup", func(t *testing.T) {
		app, router, strategy, m := NewApiTest(t)
		StartPoke(router, strategy)
		m.EXPECT().RequestSessionGroup(gomock.Any(), "test", gomock.Any()).Return(&sablier.SessionState{}, nil)
		r := PerformRequest(app, "GET", "/api/strategies/poke?group=test")
		assert.Equal(t, http.StatusOK, r.Code)
		assert.Equal(t, SablierStatusReady, r.Header().Get(SablierStatusHeader))
	})
	t.Run("StartPokeNotReadyReturnsNotReady", func(t *testing.T) {
		app, router, strategy, m := NewApiTest(t)
		StartPoke(router, strategy)
		notReady := &sablier.SessionState{
			Instances: map[string]sablier.InstanceInfoWithError{
				"test": {
					Instance: sablier.InstanceInfo{Name: "test", CurrentReplicas: 0, DesiredReplicas: 1, Status: sablier.InstanceStatusStarting},
				},
			},
		}
		m.EXPECT().RequestSessionGroup(gomock.Any(), "test", gomock.Any()).Return(notReady, nil)
		r := PerformRequest(app, "GET", "/api/strategies/poke?group=test")
		assert.Equal(t, http.StatusOK, r.Code)
		assert.Equal(t, SablierStatusNotReady, r.Header().Get(SablierStatusHeader))
	})
	t.Run("StartPokeErrGroupNotFound", func(t *testing.T) {
		app, router, strategy, m := NewApiTest(t)
		StartPoke(router, strategy)
		m.EXPECT().RequestSessionGroup(gomock.Any(), "test", gomock.Any()).Return(nil, sablier.ErrGroupNotFound{
			Group:           "test",
			AvailableGroups: []string{"test1", "test2"},
		})
		r := PerformRequest(app, "GET", "/api/strategies/poke?group=test")
		assert.Equal(t, http.StatusNotFound, r.Code)
		assert.Equal(t, rfc7807.JSONMediaType, r.Header().Get("Content-Type"))
	})
	t.Run("StartPokeError", func(t *testing.T) {
		app, router, strategy, m := NewApiTest(t)
		StartPoke(router, strategy)
		m.EXPECT().RequestSessionGroup(gomock.Any(), "test", gomock.Any()).Return(nil, errors.New("unknown error"))
		r := PerformRequest(app, "GET", "/api/strategies/poke?group=test")
		assert.Equal(t, http.StatusInternalServerError, r.Code)
		assert.Equal(t, rfc7807.JSONMediaType, r.Header().Get("Content-Type"))
	})
	t.Run("StartPokeSessionNil", func(t *testing.T) {
		app, router, strategy, m := NewApiTest(t)
		StartPoke(router, strategy)
		m.EXPECT().RequestSessionGroup(gomock.Any(), "test", gomock.Any()).Return(nil, nil)
		r := PerformRequest(app, "GET", "/api/strategies/poke?group=test")
		assert.Equal(t, http.StatusInternalServerError, r.Code)
		assert.Equal(t, rfc7807.JSONMediaType, r.Header().Get("Content-Type"))
	})
}
