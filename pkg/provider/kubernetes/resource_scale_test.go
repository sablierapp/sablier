package kubernetes

// Unit tests for buildResourcePatch.
// These run without a real Kubernetes cluster.

import (
	"encoding/json"
	"testing"

	"gotest.tools/v3/assert"
)

func TestBuildResourcePatch_CPUAndMemory(t *testing.T) {
	data, err := buildResourcePatch("mycontainer", "500m", "128Mi")
	assert.NilError(t, err)

	var doc map[string]interface{}
	assert.NilError(t, json.Unmarshal(data, &doc))

	spec := doc["spec"].(map[string]interface{})
	template := spec["template"].(map[string]interface{})
	podSpec := template["spec"].(map[string]interface{})
	containers := podSpec["containers"].([]interface{})
	assert.Equal(t, len(containers), 1)

	c := containers[0].(map[string]interface{})
	assert.Equal(t, c["name"].(string), "mycontainer")

	resources := c["resources"].(map[string]interface{})
	limits := resources["limits"].(map[string]interface{})
	assert.Equal(t, limits["cpu"].(string), "500m")
	assert.Equal(t, limits["memory"].(string), "128Mi")
}

func TestBuildResourcePatch_CPUOnly(t *testing.T) {
	data, err := buildResourcePatch("app", "0.5", "")
	assert.NilError(t, err)

	var doc map[string]interface{}
	assert.NilError(t, json.Unmarshal(data, &doc))

	spec := doc["spec"].(map[string]interface{})
	template := spec["template"].(map[string]interface{})
	podSpec := template["spec"].(map[string]interface{})
	containers := podSpec["containers"].([]interface{})
	c := containers[0].(map[string]interface{})
	resources := c["resources"].(map[string]interface{})
	limits := resources["limits"].(map[string]interface{})

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
