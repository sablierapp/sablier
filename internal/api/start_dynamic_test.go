package api

import (
	"errors"
	"github.com/sablierapp/sablier/app/instance"
	"github.com/sablierapp/sablier/app/sessions"
	"github.com/tniswong/go.rfcx/rfc7807"
	"go.uber.org/mock/gomock"
	"gotest.tools/v3/assert"
	"net/http"
	"testing"
)

func session() *sessions.SessionState {
	state := instance.ReadyInstanceState("test", 1)
	state2 := instance.ReadyInstanceState("test2", 1)
	return &sessions.SessionState{
		Instances: map[string]sessions.InstanceState{
			"test": {
				Instance: state,
				Error:    nil,
			},
			"test2": {
				Instance: state2,
				Error:    nil,
			},
		},
	}
}

func TestStartDynamic(t *testing.T) {
	t.Run("StartDynamicInvalidBind", func(t *testing.T) {
		app, router, strategy, _ := NewApiTest(t)
		StartDynamic(router, strategy)
		r := PerformRequest(app, "GET", "/api/strategies/dynamic?timeout=invalid")
		assert.Equal(t, http.StatusBadRequest, r.Code)
		assert.Equal(t, rfc7807.JSONMediaType, r.Header().Get("Content-Type"))
	})
	t.Run("StartDynamicWithoutNamesOrGroup", func(t *testing.T) {
		app, router, strategy, _ := NewApiTest(t)
		StartDynamic(router, strategy)
		r := PerformRequest(app, "GET", "/api/strategies/dynamic")
		assert.Equal(t, http.StatusBadRequest, r.Code)
		assert.Equal(t, rfc7807.JSONMediaType, r.Header().Get("Content-Type"))
	})
	t.Run("StartDynamicWithNamesAndGroup", func(t *testing.T) {
		app, router, strategy, _ := NewApiTest(t)
		StartDynamic(router, strategy)
		r := PerformRequest(app, "GET", "/api/strategies/dynamic?names=test&group=test")
		assert.Equal(t, http.StatusBadRequest, r.Code)
		assert.Equal(t, rfc7807.JSONMediaType, r.Header().Get("Content-Type"))
	})
	t.Run("StartDynamicThemeNotFound", func(t *testing.T) {
		app, router, strategy, m := NewApiTest(t)
		StartDynamic(router, strategy)
		m.EXPECT().RequestSessionGroup(gomock.Any(), "test", gomock.Any()).Return(session(), nil)
		r := PerformRequest(app, "GET", "/api/strategies/dynamic?group=test&theme=invalid")
		assert.Equal(t, http.StatusNotFound, r.Code)
		assert.Equal(t, rfc7807.JSONMediaType, r.Header().Get("Content-Type"))
	})
	t.Run("StartDynamicByNames", func(t *testing.T) {
		app, router, strategy, m := NewApiTest(t)
		StartDynamic(router, strategy)
		m.EXPECT().RequestSession(gomock.Any(), []string{"test"}, gomock.Any()).Return(session(), nil)
		r := PerformRequest(app, "GET", "/api/strategies/dynamic?names=test")
		assert.Equal(t, http.StatusOK, r.Code)
		assert.Equal(t, SablierStatusReady, r.Header().Get(SablierStatusHeader))
	})
	t.Run("StartDynamicByGroup", func(t *testing.T) {
		app, router, strategy, m := NewApiTest(t)
		StartDynamic(router, strategy)
		m.EXPECT().RequestSessionGroup(gomock.Any(), "test", gomock.Any()).Return(session(), nil)
		r := PerformRequest(app, "GET", "/api/strategies/dynamic?group=test")
		assert.Equal(t, http.StatusOK, r.Code)
		assert.Equal(t, SablierStatusReady, r.Header().Get(SablierStatusHeader))
	})
	t.Run("StartDynamicErrGroupNotFound", func(t *testing.T) {
		app, router, strategy, m := NewApiTest(t)
		StartDynamic(router, strategy)
		m.EXPECT().RequestSessionGroup(gomock.Any(), "test", gomock.Any()).Return(nil, sessions.ErrGroupNotFound{
			Group:           "test",
			AvailableGroups: []string{"test1", "test2"},
		})
		r := PerformRequest(app, "GET", "/api/strategies/dynamic?group=test")
		assert.Equal(t, http.StatusNotFound, r.Code)
		assert.Equal(t, rfc7807.JSONMediaType, r.Header().Get("Content-Type"))
	})
	t.Run("StartDynamicError", func(t *testing.T) {
		app, router, strategy, m := NewApiTest(t)
		StartDynamic(router, strategy)
		m.EXPECT().RequestSessionGroup(gomock.Any(), "test", gomock.Any()).Return(nil, errors.New("unknown error"))
		r := PerformRequest(app, "GET", "/api/strategies/dynamic?group=test")
		assert.Equal(t, http.StatusInternalServerError, r.Code)
		assert.Equal(t, rfc7807.JSONMediaType, r.Header().Get("Content-Type"))
	})
	t.Run("StartDynamicSessionNil", func(t *testing.T) {
		app, router, strategy, m := NewApiTest(t)
		StartDynamic(router, strategy)
		m.EXPECT().RequestSessionGroup(gomock.Any(), "test", gomock.Any()).Return(nil, nil)
		r := PerformRequest(app, "GET", "/api/strategies/dynamic?group=test")
		assert.Equal(t, http.StatusInternalServerError, r.Code)
		assert.Equal(t, rfc7807.JSONMediaType, r.Header().Get("Content-Type"))
	})
}
