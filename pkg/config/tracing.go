package config

// Tracing holds the OpenTelemetry tracing configuration.
type Tracing struct {
	// Enabled controls whether distributed tracing is active.
	Enabled bool `mapstructure:"ENABLED" yaml:"enabled"`

	// ExporterType selects the trace exporter backend.
	// Supported values: "otlphttp" (default), "stdout".
	ExporterType string `mapstructure:"EXPORTER_TYPE" yaml:"exporterType"`

	// Endpoint is the OTLP collector base URL (scheme + host + optional port).
	// Examples: "http://localhost:4318", "https://otel-collector:4318".
	// Only used when ExporterType is "otlphttp".
	Endpoint string `mapstructure:"ENDPOINT" yaml:"endpoint"`

	// ServiceName is the logical name of the service reported to the tracing backend.
	ServiceName string `mapstructure:"SERVICE_NAME" yaml:"serviceName"`

	// SamplingRate is the fraction of traces to sample, between 0.0 and 1.0.
	// 1.0 samples every trace (default). 0.0 samples nothing.
	SamplingRate float64 `mapstructure:"SAMPLING_RATE" yaml:"samplingRate"`
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
