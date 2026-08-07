package dockerswarm

import (
	"context"
	"log/slog"
	"testing"

	"github.com/moby/moby/api/types/swarm"
	"github.com/moby/moby/client"
	"go.opentelemetry.io/otel"
	"gotest.tools/v3/assert"
)

// fakeServiceLister implements only ServiceList; every other APIClient method
// panics through the embedded nil interface, which is fine for these tests.
type fakeServiceLister struct {
	client.APIClient
	services []swarm.Service
}

func (f fakeServiceLister) ServiceList(_ context.Context, _ client.ServiceListOptions) (client.ServiceListResult, error) {
	return client.ServiceListResult{Items: f.services}, nil
}

func newUnitProvider(services ...swarm.Service) *Provider {
	return &Provider{
		Client: fakeServiceLister{services: services},
		l:      slog.New(slog.DiscardHandler),
		tracer: otel.Tracer("test"),
	}
}

// TestInstanceInspect_SubstringMatchOnly reproduces the nil-service panic:
// Docker's "name" filter is a substring match, so inspecting "demo" while only
// "demo-app" exists returns a non-empty list with no exact match. Before the
// fix, getServiceByName returned (nil, nil) and the caller dereferenced the
// nil service (after the debug log's arguments had already done so).
func TestInstanceInspect_SubstringMatchOnly(t *testing.T) {
	t.Parallel()

	p := newUnitProvider(swarm.Service{
		ID: "abc123",
		Spec: swarm.ServiceSpec{
			Annotations: swarm.Annotations{Name: "demo-app"},
		},
	})

	_, err := p.InstanceInspect(context.Background(), "demo")
	assert.ErrorContains(t, err, "was not found")
}

// TestInstanceInspect_ExactMatchAmongSubstringMatches pins that the exact
// match still wins when the filter returns several candidates.
func TestInstanceInspect_ExactMatchAmongSubstringMatches(t *testing.T) {
	t.Parallel()

	replicas := uint64(1)
	p := newUnitProvider(
		swarm.Service{
			ID:   "aaa",
			Spec: swarm.ServiceSpec{Annotations: swarm.Annotations{Name: "demo-app"}},
		},
		swarm.Service{
			ID: "bbb",
			Spec: swarm.ServiceSpec{
				Annotations: swarm.Annotations{Name: "demo"},
				Mode:        swarm.ServiceMode{Replicated: &swarm.ReplicatedService{Replicas: &replicas}},
			},
			ServiceStatus: &swarm.ServiceStatus{DesiredTasks: 1, RunningTasks: 1},
		},
	)

	info, err := p.InstanceInspect(context.Background(), "demo")
	assert.NilError(t, err)
	assert.Equal(t, "demo", info.Name)
}
