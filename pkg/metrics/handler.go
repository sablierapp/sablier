package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// NewHandler returns an http.Handler that serves the Prometheus exposition
// format for the given recorder's registry.
func NewHandler(r *PromRecorder) http.Handler {
	return promhttp.HandlerFor(r.registry, promhttp.HandlerOpts{
		Registry: r.registry,
	})
}
