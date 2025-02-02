package api

import (
	"errors"
	"github.com/sablierapp/sablier/app/sessions"
	"github.com/tniswong/go.rfcx/rfc7807"
	"go.uber.org/mock/gomock"
	"gotest.tools/v3/assert"
	"net/http"
	"testing"
)

func TestStartBlocking(t *testing.T) {
	t.Run("StartBlockingInvalidBind", func(t *testing.T) {
		app, router, strategy, _ := NewApiTest(t)
		StartBlocking(router, strategy)
		r := PerformRequest(app, "GET", "/api/strategies/blocking?timeout=invalid")
		assert.Equal(t, http.StatusBadRequest, r.Code)
		assert.Equal(t, rfc7807.JSONMediaType, r.Header().Get("Content-Type"))
	})
	t.Run("StartBlockingWithoutNamesOrGroup", func(t *testing.T) {
		app, router, strategy, _ := NewApiTest(t)
		StartBlocking(router, strategy)
		r := PerformRequest(app, "GET", "/api/strategies/blocking")
		assert.Equal(t, http.StatusBadRequest, r.Code)
		assert.Equal(t, rfc7807.JSONMediaType, r.Header().Get("Content-Type"))
	})
	t.Run("StartBlockingWithNamesAndGroup", func(t *testing.T) {
		app, router, strategy, _ := NewApiTest(t)
		StartBlocking(router, strategy)
		r := PerformRequest(app, "GET", "/api/strategies/blocking?names=test&group=test")
		assert.Equal(t, http.StatusBadRequest, r.Code)
		assert.Equal(t, rfc7807.JSONMediaType, r.Header().Get("Content-Type"))
	})
	t.Run("StartBlockingByNames", func(t *testing.T) {
		app, router, strategy, m := NewApiTest(t)
		StartBlocking(router, strategy)
		m.EXPECT().RequestReadySession(gomock.Any(), []string{"test"}, gomock.Any(), gomock.Any()).Return(&sessions.SessionState{}, nil)
		r := PerformRequest(app, "GET", "/api/strategies/blocking?names=test")
		assert.Equal(t, http.StatusOK, r.Code)
		assert.Equal(t, SablierStatusReady, r.Header().Get(SablierStatusHeader))
	})
	t.Run("StartBlockingByGroup", func(t *testing.T) {
		app, router, strategy, m := NewApiTest(t)
		StartBlocking(router, strategy)
		m.EXPECT().RequestReadySessionGroup(gomock.Any(), "test", gomock.Any(), gomock.Any()).Return(&sessions.SessionState{}, nil)
		r := PerformRequest(app, "GET", "/api/strategies/blocking?group=test")
		assert.Equal(t, http.StatusOK, r.Code)
		assert.Equal(t, SablierStatusReady, r.Header().Get(SablierStatusHeader))
	})
	t.Run("StartBlockingErrGroupNotFound", func(t *testing.T) {
		app, router, strategy, m := NewApiTest(t)
		StartBlocking(router, strategy)
		m.EXPECT().RequestReadySessionGroup(gomock.Any(), "test", gomock.Any(), gomock.Any()).Return(nil, sessions.ErrGroupNotFound{
			Group:           "test",
			AvailableGroups: []string{"test1", "test2"},
		})
		r := PerformRequest(app, "GET", "/api/strategies/blocking?group=test")
		assert.Equal(t, http.StatusNotFound, r.Code)
		assert.Equal(t, rfc7807.JSONMediaType, r.Header().Get("Content-Type"))
	})
	t.Run("StartBlockingError", func(t *testing.T) {
		app, router, strategy, m := NewApiTest(t)
		StartBlocking(router, strategy)
		m.EXPECT().RequestReadySessionGroup(gomock.Any(), "test", gomock.Any(), gomock.Any()).Return(nil, errors.New("unknown error"))
		r := PerformRequest(app, "GET", "/api/strategies/blocking?group=test")
		assert.Equal(t, http.StatusInternalServerError, r.Code)
		assert.Equal(t, rfc7807.JSONMediaType, r.Header().Get("Content-Type"))
	})
	t.Run("StartBlockingSessionNil", func(t *testing.T) {
		app, router, strategy, m := NewApiTest(t)
		StartBlocking(router, strategy)
		m.EXPECT().RequestReadySessionGroup(gomock.Any(), "test", gomock.Any(), gomock.Any()).Return(nil, nil)
		r := PerformRequest(app, "GET", "/api/strategies/blocking?group=test")
		assert.Equal(t, http.StatusInternalServerError, r.Code)
		assert.Equal(t, rfc7807.JSONMediaType, r.Header().Get("Content-Type"))
	})
}
