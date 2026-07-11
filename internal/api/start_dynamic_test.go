package api

import (
	"errors"
	"net/http"
	"testing"
	"testing/fstest"

	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/sablier"
	"github.com/sablierapp/sablier/pkg/theme"
	"github.com/tniswong/go.rfcx/rfc7807"
	"go.uber.org/mock/gomock"
	"gotest.tools/v3/assert"
)

func session() *sablier.SessionState {
	state := sablier.InstanceInfo{Name: "test", CurrentReplicas: 1, DesiredReplicas: 1, Status: sablier.InstanceStatusReady}
	state2 := sablier.InstanceInfo{Name: "test2", CurrentReplicas: 1, DesiredReplicas: 1, Status: sablier.InstanceStatusReady}
	return &sablier.SessionState{
		Instances: map[string]sablier.InstanceInfoWithError{
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
		app, router, strategy, _ := NewApiTest(t)
		StartDynamic(router, strategy)
		// No session expectation on purpose: an unknown theme must be rejected
		// BEFORE any instance is started (the request can only ever 404, and
		// every retry would otherwise start the workloads again).
		r := PerformRequest(app, "GET", "/api/strategies/dynamic?group=test&theme=invalid")
		assert.Equal(t, http.StatusNotFound, r.Code)
		assert.Equal(t, rfc7807.JSONMediaType, r.Header().Get("Content-Type"))
	})
	t.Run("StartDynamicRenderError", func(t *testing.T) {
		app, router, strategy, m := NewApiTest(t)
		// A theme that parses fine but fails at execution time (field access on
		// a string): easy to author in a custom theme, must surface as a 500
		// problem instead of a 200 with a broken page.
		badFS := fstest.MapFS{"bad.html": &fstest.MapFile{Data: []byte(`{{ .DisplayName.Bad }}`)}}
		th, err := theme.NewWithCustomThemes(badFS, slogt.New(t))
		assert.NilError(t, err)
		strategy.Theme = th
		StartDynamic(router, strategy)
		m.EXPECT().RequestSession(gomock.Any(), []string{"test"}, gomock.Any()).Return(session(), nil)
		r := PerformRequest(app, "GET", "/api/strategies/dynamic?names=test&theme=bad")
		assert.Equal(t, http.StatusInternalServerError, r.Code)
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
		m.EXPECT().RequestSessionGroup(gomock.Any(), "test", gomock.Any()).Return(nil, sablier.ErrGroupNotFound{
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
