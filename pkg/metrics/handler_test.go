package metrics_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sablierapp/sablier/pkg/metrics"
)

func TestNewHandler_ServesPrometheusExposition(t *testing.T) {
	r := metrics.NewPromRecorder()
	r.RecordSessionRequest("dynamic", "names")

	srv := httptest.NewServer(metrics.NewHandler(r))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/plain") && !strings.HasPrefix(ct, "application/openmetrics-text") {
		t.Fatalf("Content-Type = %q, want text/plain or openmetrics", ct)
	}

	body, _ := io.ReadAll(resp.Body)
	bs := string(body)
	if !strings.Contains(bs, "sablier_session_requests_total") {
		t.Errorf("body missing sablier_session_requests_total, got:\n%s", bs)
	}
	if !strings.Contains(bs, "go_goroutines") {
		t.Errorf("body missing go_goroutines, got:\n%s", bs)
	}
}
