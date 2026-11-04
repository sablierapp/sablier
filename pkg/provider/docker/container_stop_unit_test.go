package docker

// Unit tests for InstanceStop. These run without a real Docker daemon.

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
	"go.opentelemetry.io/otel/trace/noop"
	"gotest.tools/v3/assert"
)

// stubInspectClient answers ContainerInspect with a fixed container. Any other
// call (stop, pause, update) hits the nil embedded client and panics, so a test
// fails if InstanceStop touches the container.
type stubInspectClient struct {
	client.APIClient
	labels map[string]string
	paused bool
}

func (c stubInspectClient) ContainerInspect(context.Context, string, client.ContainerInspectOptions) (client.ContainerInspectResult, error) {
	return client.ContainerInspectResult{Container: container.InspectResponse{
		Config: &container.Config{Labels: c.labels},
		State:  &container.State{Running: true, Paused: c.paused},
	}}, nil
}

func TestInstanceStop_ReportsNoopWhenNothingToDo(t *testing.T) {
	tests := []struct {
		name     string
		strategy string
		labels   map[string]string
		paused   bool
	}{
		{
			name:     "scale mode with idle replicas and no idle resources",
			strategy: "stop",
			labels:   map[string]string{sablier.LabelEnable: "true", sablier.LabelIdleReplicas: "1"},
		},
		{
			name:     "already paused",
			strategy: "pause",
			labels:   map[string]string{sablier.LabelEnable: "true"},
			paused:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Provider{
				Client:   stubInspectClient{labels: tt.labels, paused: tt.paused},
				l:        slog.New(slog.NewTextHandler(io.Discard, nil)),
				strategy: tt.strategy,
				tracer:   noop.NewTracerProvider().Tracer("test"),
			}
			err := p.InstanceStop(t.Context(), "plex")
			assert.ErrorIs(t, err, provider.ErrNoop)
		})
	}
}
