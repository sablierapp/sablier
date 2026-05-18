package config

// Tracing holds the OpenTelemetry tracing configuration.
type Tracing struct {
	// Enabled activates distributed tracing. When false, no spans are created or exported.
	// Env: SABLIER_TRACING_ENABLED
	// CLI: --tracing.enabled
	// Default: false
	Enabled bool

	// ExporterType selects the trace exporter backend.
	// Accepted values: "otlphttp" (default), "stdout".
	// Env: SABLIER_TRACING_EXPORTER_TYPE
	// CLI: --tracing.exporter-type
	// Default: "otlphttp"
	ExporterType string

	// Endpoint is the OTLP collector base URL (scheme + host + optional port).
	// Examples: "http://jaeger:4318", "http://tempo:4318".
	// Only used when ExporterType is "otlphttp".
	// Env: SABLIER_TRACING_ENDPOINT
	// CLI: --tracing.endpoint
	// Default: "http://localhost:4318"
	Endpoint string

	// ServiceName is the logical name of this service as it appears in the tracing backend UI.
	// Env: SABLIER_TRACING_SERVICE_NAME
	// CLI: --tracing.service-name
	// Default: "sablier"
	ServiceName string

	// SamplingRate is the fraction of requests to trace, from 0.0 (none) to 1.0 (all).
	// Env: SABLIER_TRACING_SAMPLING_RATE
	// CLI: --tracing.sampling-rate
	// Default: 1.0
	SamplingRate float64
}

func NewTracingConfig() Tracing {
	return Tracing{
		Enabled:      false,
		ExporterType: "otlphttp",
		Endpoint:     "http://localhost:4318",
		ServiceName:  "sablier",
		SamplingRate: 1.0,
	}
}
