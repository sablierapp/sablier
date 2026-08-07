package kubernetes

// Unit tests for buildResourcePatch and resource scaling helpers.
// These run without a real Kubernetes cluster.

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"gotest.tools/v3/assert"
	"k8s.io/client-go/kubernetes/fake"
)

func TestBuildResourcePatch_CPUAndMemory(t *testing.T) {
	data, err := buildResourcePatch("mycontainer", "500m", "128Mi")
	assert.NilError(t, err)

	var doc map[string]any
	assert.NilError(t, json.Unmarshal(data, &doc))

	spec := doc["spec"].(map[string]any)
	template := spec["template"].(map[string]any)
	podSpec := template["spec"].(map[string]any)
	containers := podSpec["containers"].([]any)
	assert.Equal(t, len(containers), 1)

	c := containers[0].(map[string]any)
	assert.Equal(t, c["name"].(string), "mycontainer")

	resources := c["resources"].(map[string]any)
	limits := resources["limits"].(map[string]any)
	assert.Equal(t, limits["cpu"].(string), "500m")
	assert.Equal(t, limits["memory"].(string), "128Mi")
}

func TestBuildResourcePatch_CPUOnly(t *testing.T) {
	data, err := buildResourcePatch("app", "0.5", "")
	assert.NilError(t, err)

	var doc map[string]any
	assert.NilError(t, json.Unmarshal(data, &doc))

	spec := doc["spec"].(map[string]any)
	template := spec["template"].(map[string]any)
	podSpec := template["spec"].(map[string]any)
	containers := podSpec["containers"].([]any)
	c := containers[0].(map[string]any)
	resources := c["resources"].(map[string]any)
	limits := resources["limits"].(map[string]any)

	// CPU should be set, memory should not appear in the patch
	_, hasCPU := limits["cpu"]
	assert.Assert(t, hasCPU, "cpu should be in limits")
	_, hasMemory := limits["memory"]
	assert.Assert(t, !hasMemory, "memory should not appear when empty")
}

func TestBuildResourcePatch_InvalidCPU(t *testing.T) {
	_, err := buildResourcePatch("app", "bad-cpu", "")
	assert.Assert(t, err != nil, "expected error for invalid CPU")
}

func TestBuildResourcePatch_InvalidMemory(t *testing.T) {
	_, err := buildResourcePatch("app", "", "bad-memory")
	assert.Assert(t, err != nil, "expected error for invalid memory")
}

func newFakeProvider() *Provider {
	return &Provider{
		Client:    fake.NewSimpleClientset(),
		delimiter: "_",
		l:         slog.Default(),
	}
}

func TestScaleResources_UnsupportedKind(t *testing.T) {
	p := newFakeProvider()
	err := p.scaleResources(context.Background(), ParsedName{Kind: "daemonset"}, "100m", "")
	assert.ErrorContains(t, err, "unsupported kind")
}

func TestGetWorkloadLabels_UnsupportedKind(t *testing.T) {
	p := newFakeProvider()
	_, err := p.getWorkloadLabels(context.Background(), ParsedName{Kind: "daemonset"})
	assert.ErrorContains(t, err, "unsupported kind")
}
