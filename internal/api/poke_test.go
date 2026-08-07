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

func TestPoke(t *testing.T) {
	t.Run("PokeInvalidBind", func(t *testing.T) {
		app, router, strategy, _ := NewApiTest(t)
		Poke(router, strategy)
		r := PerformRequest(app, "GET", "/api/poke?session_duration=invalid")
		assert.Equal(t, http.StatusBadRequest, r.Code)
		assert.Equal(t, rfc7807.JSONMediaType, r.Header().Get("Content-Type"))
	})

	t.Run("PokeWithoutNamesOrGroup", func(t *testing.T) {
		app, router, strategy, _ := NewApiTest(t)
		Poke(router, strategy)
		r := PerformRequest(app, "GET", "/api/poke")
		assert.Equal(t, http.StatusBadRequest, r.Code)
		assert.Equal(t, rfc7807.JSONMediaType, r.Header().Get("Content-Type"))
	})

	t.Run("PokeWithNamesAndGroup", func(t *testing.T) {
		app, router, strategy, _ := NewApiTest(t)
		Poke(router, strategy)
		r := PerformRequest(app, "GET", "/api/poke?names=test&group=test")
		assert.Equal(t, http.StatusBadRequest, r.Code)
		assert.Equal(t, rfc7807.JSONMediaType, r.Header().Get("Content-Type"))
	})

	t.Run("PokeByNames", func(t *testing.T) {
		app, router, strategy, m := NewApiTest(t)
		Poke(router, strategy)
		m.EXPECT().ExtendSession(gomock.Any(), []string{"nginx"}, gomock.Any()).
			Return(&sablier.SessionState{
				Instances: map[string]sablier.InstanceInfoWithError{
					"nginx": {Instance: sablier.InstanceInfo{Name: "nginx", Status: sablier.InstanceStatusReady}},
				},
			}, nil)
		r := PerformRequest(app, "GET", "/api/poke?names=nginx")
		assert.Equal(t, http.StatusOK, r.Code)
		assert.Equal(t, SablierStatusReady, r.Header().Get(SablierStatusHeader))
	})

	t.Run("PokeByGroup", func(t *testing.T) {
		app, router, strategy, m := NewApiTest(t)
		Poke(router, strategy)
		m.EXPECT().ExtendSessionGroup(gomock.Any(), "mygroup", gomock.Any()).
			Return(&sablier.SessionState{}, nil)
		r := PerformRequest(app, "GET", "/api/poke?group=mygroup")
		assert.Equal(t, http.StatusOK, r.Code)
	})

	t.Run("PokeErrGroupNotFound", func(t *testing.T) {
		app, router, strategy, m := NewApiTest(t)
		Poke(router, strategy)
		m.EXPECT().ExtendSessionGroup(gomock.Any(), "missing", gomock.Any()).
			Return(nil, sablier.ErrGroupNotFound{
				Group:           "missing",
				AvailableGroups: []string{"mygroup"},
			})
		r := PerformRequest(app, "GET", "/api/poke?group=missing")
		assert.Equal(t, http.StatusNotFound, r.Code)
		assert.Equal(t, rfc7807.JSONMediaType, r.Header().Get("Content-Type"))
	})

	t.Run("PokeError", func(t *testing.T) {
		app, router, strategy, m := NewApiTest(t)
		Poke(router, strategy)
		m.EXPECT().ExtendSession(gomock.Any(), []string{"nginx"}, gomock.Any()).
			Return(nil, errors.New("store unavailable"))
		r := PerformRequest(app, "GET", "/api/poke?names=nginx")
		assert.Equal(t, http.StatusInternalServerError, r.Code)
		assert.Equal(t, rfc7807.JSONMediaType, r.Header().Get("Content-Type"))
	})
}
