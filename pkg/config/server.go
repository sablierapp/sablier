package config

// Server holds the HTTP server configuration.
type Server struct {
	// Port is the TCP port the Sablier server listens on.
	// Env: SABLIER_SERVER_PORT
	// CLI: --server.port
	// Default: 10000
	Port int

	// BasePath is the URL path prefix for all API routes.
	// Useful when Sablier is served behind a reverse proxy at a sub-path.
	// Env: SABLIER_SERVER_BASE_PATH
	// CLI: --server.base-path
	// Default: "/"
	BasePath string

	Metrics MetricsConfig
}

// MetricsConfig controls the Prometheus metrics endpoint.
type MetricsConfig struct {
	// Enabled exposes a Prometheus-compatible /metrics endpoint when true.
	// Env: SABLIER_SERVER_METRICS_ENABLED
	// CLI: --server.metrics.enabled
	// Default: false
	Enabled bool
}

func NewServerConfig() Server {
	return Server{
		Port:     10000,
		BasePath: "/",
		Metrics:  MetricsConfig{Enabled: false},
	}
}
