package server

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/internal/api"
	"github.com/sablierapp/sablier/pkg/config"
	"github.com/sablierapp/sablier/pkg/metrics"
)

func TestMetricsEndpoint_EnabledServesPrometheusExposition(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := metrics.NewPromRecorder()
	rec.RecordSessionRequest("dynamic", "names")

	strategy := &api.ServeStrategy{
		Metrics: rec,
	}
	r := setupRouter(context.Background(), slogt.New(t), config.Server{BasePath: "/"}, strategy)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/metrics", nil)
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "sablier_session_requests_total") {
		t.Errorf("body missing sablier_session_requests_total; got:\n%s", body)
	}
	if !strings.Contains(body, "go_goroutines") {
		t.Errorf("body missing go_goroutines; got:\n%s", body)
	}
}

func TestMetricsEndpoint_DisabledReturns404(t *testing.T) {
	gin.SetMode(gin.TestMode)

	strategy := &api.ServeStrategy{
		Metrics: metrics.Noop{},
	}
	r := setupRouter(context.Background(), slogt.New(t), config.Server{BasePath: "/"}, strategy)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/metrics", nil)
	r.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestMetricsEndpoint_RespectsBasePath(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := metrics.NewPromRecorder()
	strategy := &api.ServeStrategy{Metrics: rec}
	r := setupRouter(context.Background(), slogt.New(t), config.Server{BasePath: "/sablier"}, strategy)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/sablier/metrics", nil)
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}
