package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/gin-gonic/gin"
	"github.com/sablierapp/sablier/internal/api/apitest"
	"github.com/sablierapp/sablier/pkg/sablier"
	"go.uber.org/mock/gomock"
	"gotest.tools/v3/assert"
)

func init() {
	// The dynamic strategy serves its waiting page as text/html, which
	// openapi3filter has no decoder for; validate it as a plain string.
	openapi3filter.RegisterBodyDecoder("text/html", openapi3filter.PlainBodyDecoder)
}

// TestOpenAPIContract replays every documented response of the strategy
// endpoints and validates status, headers, media type and body against the
// published spec (docs/static/openapi.json). Generated clients are built from
// that spec, so any response it does not describe would surface in a client
// as an undecodable body or an unexpected status.
func TestOpenAPIContract(t *testing.T) {
	doc, err := openapi3.NewLoader().LoadFromFile("../../docs/static/openapi.json")
	assert.NilError(t, err)
	assert.NilError(t, doc.Validate(context.Background()))

	notReady := &sablier.SessionState{
		Instances: map[string]sablier.InstanceInfoWithError{
			"web": {Instance: sablier.InstanceInfo{Name: "web", Status: sablier.InstanceStatusStarting, DesiredReplicas: 1}},
		},
	}
	groupNotFound := sablier.ErrGroupNotFound{Group: "test", AvailableGroups: []string{"other"}}

	tests := []struct {
		name   string
		start  func(*gin.RouterGroup, *ServeStrategy)
		target string
		mock   func(m *apitest.MockSablier)
		status int
	}{
		{name: "dynamic ok", start: StartDynamic, target: "/api/strategies/dynamic?names=web", status: http.StatusOK,
			mock: func(m *apitest.MockSablier) {
				m.EXPECT().RequestSession(gomock.Any(), gomock.Any(), gomock.Any()).Return(notReady, nil)
			}},
		{name: "dynamic validation", start: StartDynamic, target: "/api/strategies/dynamic", status: http.StatusBadRequest},
		{name: "dynamic theme not found", start: StartDynamic, target: "/api/strategies/dynamic?names=web&theme=nope", status: http.StatusNotFound},
		{name: "dynamic group not found", start: StartDynamic, target: "/api/strategies/dynamic?group=test", status: http.StatusNotFound,
			mock: func(m *apitest.MockSablier) {
				m.EXPECT().RequestSessionGroup(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, groupNotFound)
			}},
		{name: "dynamic error", start: StartDynamic, target: "/api/strategies/dynamic?names=web", status: http.StatusInternalServerError,
			mock: func(m *apitest.MockSablier) {
				m.EXPECT().RequestSession(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("boom"))
			}},

		{name: "blocking ok", start: StartBlocking, target: "/api/strategies/blocking?names=web", status: http.StatusOK,
			mock: func(m *apitest.MockSablier) {
				m.EXPECT().RequestReadySession(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(notReady, nil)
			}},
		{name: "blocking validation", start: StartBlocking, target: "/api/strategies/blocking", status: http.StatusBadRequest},
		{name: "blocking group not found", start: StartBlocking, target: "/api/strategies/blocking?group=test", status: http.StatusNotFound,
			mock: func(m *apitest.MockSablier) {
				m.EXPECT().RequestReadySessionGroup(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, groupNotFound)
			}},
		{name: "blocking not managed", start: StartBlocking, target: "/api/strategies/blocking?names=web", status: http.StatusNotFound,
			mock: func(m *apitest.MockSablier) {
				m.EXPECT().RequestReadySession(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, sablier.ErrInstanceNotManaged{Name: "web"})
			}},
		{name: "blocking error", start: StartBlocking, target: "/api/strategies/blocking?names=web", status: http.StatusInternalServerError,
			mock: func(m *apitest.MockSablier) {
				m.EXPECT().RequestReadySession(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("boom"))
			}},
		{name: "blocking cancelled", start: StartBlocking, target: "/api/strategies/blocking?names=web", status: http.StatusServiceUnavailable,
			mock: func(m *apitest.MockSablier) {
				m.EXPECT().RequestReadySession(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, context.Canceled)
			}},
		{name: "blocking timeout", start: StartBlocking, target: "/api/strategies/blocking?names=web", status: http.StatusGatewayTimeout,
			mock: func(m *apitest.MockSablier) {
				m.EXPECT().RequestReadySession(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, sablier.ErrTimeout{Duration: time.Minute, Instances: []sablier.InstanceInfoWithError{notReady.Instances["web"]}})
			}},

		{name: "poke ok", start: StartPoke, target: "/api/strategies/poke?names=web", status: http.StatusOK,
			mock: func(m *apitest.MockSablier) {
				m.EXPECT().RequestSession(gomock.Any(), gomock.Any(), gomock.Any()).Return(notReady, nil)
			}},
		{name: "poke validation", start: StartPoke, target: "/api/strategies/poke", status: http.StatusBadRequest},
		{name: "poke group not found", start: StartPoke, target: "/api/strategies/poke?group=test", status: http.StatusNotFound,
			mock: func(m *apitest.MockSablier) {
				m.EXPECT().RequestSessionGroup(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, groupNotFound)
			}},
		{name: "poke error", start: StartPoke, target: "/api/strategies/poke?names=web", status: http.StatusInternalServerError,
			mock: func(m *apitest.MockSablier) {
				m.EXPECT().RequestSession(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("boom"))
			}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, router, strategy, m := NewApiTest(t)
			tt.start(router, strategy)
			if tt.mock != nil {
				tt.mock(m)
			}

			req := httptest.NewRequest(http.MethodGet, tt.target, nil)
			rec := httptest.NewRecorder()
			app.ServeHTTP(rec, req)
			assert.Equal(t, tt.status, rec.Code, rec.Body.String())

			pathItem := doc.Paths.Find(req.URL.Path)
			assert.Assert(t, pathItem != nil, "path %s is not documented", req.URL.Path)
			route := &routers.Route{Spec: doc, Path: req.URL.Path, PathItem: pathItem, Method: req.Method, Operation: pathItem.Get}

			err := openapi3filter.ValidateResponse(context.Background(), &openapi3filter.ResponseValidationInput{
				RequestValidationInput: &openapi3filter.RequestValidationInput{Request: req, Route: route},
				Status:                 rec.Code,
				Header:                 rec.Header(),
				Body:                   io.NopCloser(rec.Body),
				Options:                &openapi3filter.Options{IncludeResponseStatus: true},
			})
			assert.NilError(t, err)
		})
	}
}
