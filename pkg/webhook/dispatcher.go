// Package webhook delivers normalized HTTP notifications when Sablier-managed
// instances change state. It acts as an abstract, provider-agnostic event
// bridge: regardless of whether the underlying runtime is Docker, Kubernetes,
// Docker Swarm, Podman, or Proxmox LXC, every consumer receives the same JSON
// payload structure.
//
// Two kinds of events are delivered on separate watches:
//
//   - Lifecycle events ("started"/"stopped") are observations of an actual
//     scale transition. They are delivered best-effort and fan out per endpoint
//     (Watch/dispatch); an endpoint with no events filter receives all of them.
//   - Intent events ("activate"/"deactivate") are emitted only for
//     delegated-scaling instances, whose replica count is owned by an external
//     scaler. They are delivered strictly in order with bounded retries
//     (WatchOrdered/dispatchOrdered) and require an explicit subscription: an
//     endpoint receives them only if its events filter lists them, so existing
//     unfiltered endpoints see no new traffic.
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
	// Event is the normalized event type. Lifecycle events are "started" or
	// "stopped"; delegated-scaling intent events are "activate" or "deactivate".
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

// Watch consumes events from stream and delivers webhooks for the lifecycle
// "started" and "stopped" transitions only. It returns when ctx is cancelled,
// the event channel is closed, or a terminal error is received from the stream.
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

// orderedDeliveryAttempts bounds how many times WatchOrdered retries a single
// delivery before giving up and moving on to the next event.
const orderedDeliveryAttempts = 5

// orderedRetryBaseDelay is the base backoff between ordered-delivery retries; it
// grows linearly with the attempt number.
const orderedRetryBaseDelay = 200 * time.Millisecond

// WatchOrdered consumes events from stream and delivers them strictly in order,
// one event at a time, retrying each delivery with bounded backoff. It is the
// counterpart to Watch for intent events (activate/deactivate) where delivery is
// load-bearing: a reordered deactivate→activate or a silently dropped delivery
// would leave the external scaler in the wrong state.
//
// Unlike Watch it does not fan out per endpoint concurrently — all of an event's
// endpoints are delivered (with retries) before the next event is read, so the
// receiver observes events in the exact order Sablier emitted them.
//
// Call WatchOrdered in a dedicated goroutine.
func (d *Dispatcher) WatchOrdered(ctx context.Context, stream sablier.InstanceEventStream) {
	eventsC := stream.Events
	errC := stream.Err

	for {
		select {
		case <-ctx.Done():
			d.l.InfoContext(ctx, "ordered webhook dispatcher stopped", slog.Any("reason", ctx.Err()))
			return
		case event, ok := <-eventsC:
			if !ok {
				d.l.WarnContext(ctx, "ordered webhook event stream closed")
				return
			}
			d.dispatchOrdered(ctx, event)
		case err, ok := <-errC:
			if !ok {
				return
			}
			d.l.ErrorContext(ctx, "ordered webhook event stream error", slog.Any("error", err))
			return
		}
	}
}

// dispatchOrdered delivers a single event to every endpoint that should fire,
// sequentially and with bounded retries, before returning.
func (d *Dispatcher) dispatchOrdered(ctx context.Context, event sablier.InstanceEvent) {
	payload := Payload{
		Event:     string(event.Type),
		Instance:  InstancePayload{Name: event.Info.Name},
		Timestamp: time.Now().UTC(),
	}
	for _, ep := range d.endpoints {
		// Intent events require an EXPLICIT subscription. Unlike shouldFire there
		// is no empty-means-all fallback: an endpoint with no events filter never
		// receives activate/deactivate, so existing unfiltered endpoints (e.g. an
		// uptime monitor watching started/stopped) see zero new traffic.
		if !slices.Contains(ep.Events, string(event.Type)) {
			continue
		}
		d.sendWithRetry(ctx, ep, payload)
	}
}

// sendWithRetry calls send up to orderedDeliveryAttempts times, backing off
// between attempts, until send reports success or the context is cancelled.
func (d *Dispatcher) sendWithRetry(ctx context.Context, ep config.WebhookEndpoint, payload Payload) {
	for attempt := 1; attempt <= orderedDeliveryAttempts; attempt++ {
		if err := d.send(ctx, ep, payload); err == nil {
			return
		}
		if attempt == orderedDeliveryAttempts {
			d.l.ErrorContext(ctx, "webhook: giving up after retries",
				slog.String("url", ep.URL),
				slog.String("event", payload.Event),
				slog.String("instance", payload.Instance.Name),
				slog.Int("attempts", attempt),
			)
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Duration(attempt) * orderedRetryBaseDelay):
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
			// Best-effort delivery: send logs its own failures, and this path
			// intentionally does not retry, so the returned error is ignored.
			_ = d.send(ctx, ep, payload)
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

// send performs the HTTP POST. It returns nil on success and an error on
// failure. Callers on the best-effort path (Watch/dispatch) ignore the return;
// the ordered path (WatchOrdered/sendWithRetry) uses it to drive retries.
func (d *Dispatcher) send(ctx context.Context, ep config.WebhookEndpoint, payload Payload) error {
	body, err := json.Marshal(payload)
	if err != nil {
		d.l.ErrorContext(ctx, "webhook: failed to marshal payload", slog.String("url", ep.URL), slog.Any("error", err))
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ep.URL, bytes.NewReader(body))
	if err != nil {
		d.l.ErrorContext(ctx, "webhook: failed to build request", slog.String("url", ep.URL), slog.Any("error", err))
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", d.userAgent)
	for k, v := range ep.Headers {
		req.Header.Set(k, v)
	}

	resp, err := d.client.Do(req)
	if err != nil {
		d.l.ErrorContext(ctx, "webhook: delivery failed", slog.String("url", ep.URL), slog.Any("error", err))
		return err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode >= 400 {
		d.l.WarnContext(ctx, "webhook: endpoint returned error status",
			slog.String("url", ep.URL),
			slog.Int("status", resp.StatusCode),
			slog.String("event", payload.Event),
			slog.String("instance", payload.Instance.Name),
		)
		return fmt.Errorf("webhook %s returned status %d", ep.URL, resp.StatusCode)
	}
	d.l.InfoContext(ctx, "webhook: delivered",
		slog.String("url", ep.URL),
		slog.Int("status", resp.StatusCode),
		slog.String("event", payload.Event),
		slog.String("instance", payload.Instance.Name),
	)
	return nil
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
