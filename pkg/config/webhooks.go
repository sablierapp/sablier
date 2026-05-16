package config

// Webhooks holds the outbound webhook notification configuration.
// Sablier fires an HTTP POST to every configured endpoint whenever an
// instance transitions to "started" or "stopped".
type Webhooks struct {
	// Endpoints is the list of HTTP targets to notify.
	Endpoints []WebhookEndpoint `mapstructure:"ENDPOINTS" yaml:"endpoints,omitempty"`
}

// WebhookEndpoint describes a single HTTP notification target.
type WebhookEndpoint struct {
	// URL is the full HTTP(S) URL to POST events to (required).
	URL string `mapstructure:"URL" yaml:"url"`
	// Headers is an optional map of HTTP request headers to include
	// (e.g. Authorization, X-Custom-Header).
	Headers map[string]string `mapstructure:"HEADERS" yaml:"headers,omitempty"`
	// Events restricts which event types trigger this endpoint.
	// Accepted values: "started", "stopped".
	// If empty or omitted, the endpoint receives all events.
	Events []string `mapstructure:"EVENTS" yaml:"events,omitempty"`
}

func NewWebhooksConfig() Webhooks {
	return Webhooks{}
}
