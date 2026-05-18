package config

// Webhooks holds the outbound webhook notification configuration.
// Sablier fires an HTTP POST to every configured endpoint whenever an
// instance transitions to "started" or "stopped".
// Webhooks are configured via the YAML configuration file only;
// there is no corresponding CLI flag or environment variable.
type Webhooks struct {
	// Endpoints is the list of HTTP targets to notify on instance lifecycle events.
	Endpoints []WebhookEndpoint
}

// WebhookEndpoint describes a single HTTP notification target.
type WebhookEndpoint struct {
	// URL is the full HTTP(S) address to POST events to. Required.
	URL string

	// Headers is an optional map of HTTP request headers added to every delivery
	// (e.g. {"Authorization": "Bearer <token>"}).
	Headers map[string]string

	// Events restricts which lifecycle events trigger a delivery to this endpoint.
	// Accepted values: "started", "stopped".
	// Omit or leave empty to receive all events.
	Events []string
}

func NewWebhooksConfig() Webhooks {
	return Webhooks{}
}
