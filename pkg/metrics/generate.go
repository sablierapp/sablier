package metrics

// Regenerate the Metrics reference page from the Prometheus collectors defined
// in this package. Kept in sync via `go generate ./...` or `make metrics-docs`.
//go:generate go run ../../cmd/metricsgen -out ../../docs/content/how-to-guides/advanced/observability/metrics.md
