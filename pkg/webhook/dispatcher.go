// Package webhook delivers normalized HTTP notifications when Sablier-managed
// instances change state. It acts as an abstract, provider-agnostic event
// bridge: regardless of whether the underlying runtime is Docker, Kubernetes,
// Docker Swarm, Podman, or Proxmox LXC, every consumer receives the same JSON
// payload structure.
package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"sync"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/sablierapp/sablier/pkg/config"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
	"github.com/sablierapp/sablier/pkg/version"
)

// Payload is the normalized JSON body posted to every webhook endpoint.
// The same structure is used regardless of the underlying provider.
type Payload struct {
	// Event is the normalized event type: "started" or "stopped".
	Event string `json:"event"`
	// Instance holds provider-agnostic instance identifiers.
	Instance InstancePayload `json:"instance"`
	// Timestamp is the UTC time at which the event was processed by Sablier.
	Timestamp time.Time `json:"timestamp"`
}

// InstancePayload contains the provider-agnostic fields for an instance.
type InstancePayload struct {
	// Name is the container / service / workload name as known to Sablier.
	Name string `json:"name"`
}

// Dispatcher watches the provider event stream and fires HTTP POST requests
// to every configured endpoint. It is intentionally stateless: each Watch
// call subscribes independently and stops when the context is cancelled.
type Dispatcher struct {
	endpoints []config.WebhookEndpoint
	client    *http.Client
	userAgent string
	l         *slog.Logger
}

// NewDispatcher creates a Dispatcher for the given endpoints.
func NewDispatcher(endpoints []config.WebhookEndpoint, logger *slog.Logger) *Dispatcher {
	v := version.Version
	if v == "" {
		v = "dev"
	}
	return &Dispatcher{
		endpoints: endpoints,
		client: &http.Client{
			Timeout:   10 * time.Second,
			Transport: otelhttp.NewTransport(http.DefaultTransport),
		},
		userAgent: "sablier/" + v,
		l:         logger,
	}
}

// Watch consumes events from stream and delivers webhooks for "started" and
// "stopped" transitions. It returns when ctx is cancelled, the event channel
// is closed, or a terminal error is received from the stream.
//
// Watch waits for all in-flight HTTP deliveries to complete before returning,
// so the caller can rely on no goroutines escaping after Watch exits.
//
// Call Watch in a dedicated goroutine.
func (d *Dispatcher) Watch(ctx context.Context, stream sablier.InstanceEventStream) {
	var wg sync.WaitGroup
	defer wg.Wait() // drain all in-flight sends before returning

	eventsC := stream.Events
	errC := stream.Err

	for {
		select {
		case <-ctx.Done():
			d.l.InfoContext(ctx, "webhook dispatcher stopped", slog.Any("reason", ctx.Err()))
			return
		case event, ok := <-eventsC:
			if !ok {
				d.l.WarnContext(ctx, "webhook event stream closed")
				return
			}
			// Only dispatch on lifecycle transitions relevant to the feature.
			if event.Type != provider.InstanceEventStarted && event.Type != provider.InstanceEventStopped {
				continue
			}
			d.dispatch(ctx, &wg, event)
		case err, ok := <-errC:
			if !ok {
				return
			}
			d.l.ErrorContext(ctx, "webhook event stream error", slog.Any("error", err))
			return
		}
	}
}

// dispatch builds the payload and spawns a goroutine per endpoint that should fire.
// Each goroutine is tracked in wg so Watch can wait for them to drain.
func (d *Dispatcher) dispatch(ctx context.Context, wg *sync.WaitGroup, event sablier.InstanceEvent) {
	payload := Payload{
		Event:     string(event.Type),
		Instance:  InstancePayload{Name: event.Info.Name},
		Timestamp: time.Now().UTC(),
	}
	for _, ep := range d.endpoints {
		if !d.shouldFire(ep, string(event.Type)) {
			continue
		}
		wg.Go(func() {
			d.send(ctx, ep, payload)
		})
	}
}

// shouldFire returns true when endpoint ep is configured to receive eventType.
func (d *Dispatcher) shouldFire(ep config.WebhookEndpoint, eventType string) bool {
	if len(ep.Events) == 0 {
		return true // no filter → all events
	}
	return slices.Contains(ep.Events, eventType)
}

// send performs the HTTP POST. Errors are logged but never returned; webhook
// delivery is best-effort and must not block the event loop.
func (d *Dispatcher) send(ctx context.Context, ep config.WebhookEndpoint, payload Payload) {
	body, err := json.Marshal(payload)
	if err != nil {
		d.l.ErrorContext(ctx, "webhook: failed to marshal payload", slog.String("url", ep.URL), slog.Any("error", err))
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ep.URL, bytes.NewReader(body))
	if err != nil {
		d.l.ErrorContext(ctx, "webhook: failed to build request", slog.String("url", ep.URL), slog.Any("error", err))
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", d.userAgent)
	for k, v := range ep.Headers {
		req.Header.Set(k, v)
	}

	resp, err := d.client.Do(req)
	if err != nil {
		d.l.ErrorContext(ctx, "webhook: delivery failed", slog.String("url", ep.URL), slog.Any("error", err))
		return
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode >= 400 {
		d.l.WarnContext(ctx, "webhook: endpoint returned error status",
			slog.String("url", ep.URL),
			slog.Int("status", resp.StatusCode),
			slog.String("event", payload.Event),
			slog.String("instance", payload.Instance.Name),
		)
		return
	}
	d.l.InfoContext(ctx, "webhook: delivered",
		slog.String("url", ep.URL),
		slog.Int("status", resp.StatusCode),
		slog.String("event", payload.Event),
		slog.String("instance", payload.Instance.Name),
	)
}

// validateEndpoints checks each endpoint for a non-empty URL.
// It returns a descriptive error for the first invalid entry found.
func validateEndpoints(endpoints []config.WebhookEndpoint) error {
	for i, ep := range endpoints {
		if ep.URL == "" {
			return fmt.Errorf("webhooks.endpoints[%d]: url is required", i)
		}
	}
	return nil
}

// Validate returns an error if any endpoint in d is misconfigured.
func (d *Dispatcher) Validate() error {
	return validateEndpoints(d.endpoints)
}
