package api

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/sablierapp/sablier/pkg/sablier"
	"gotest.tools/v3/assert"
)

func TestNewSessionResponse(t *testing.T) {
	t.Run("sorted instances with status and error string", func(t *testing.T) {
		s := &sablier.SessionState{
			Instances: map[string]sablier.InstanceInfoWithError{
				"b": {Instance: sablier.InstanceInfo{Name: "b", Status: sablier.InstanceStatusReady}},
				"a": {Instance: sablier.InstanceInfo{Name: "a"}, Error: errors.New("provider unavailable")},
			},
		}

		resp := NewSessionResponse(s)

		assert.Equal(t, "not-ready", resp.Session.Status)
		assert.Equal(t, 2, len(resp.Session.Instances))
		assert.Equal(t, "a", resp.Session.Instances[0].Instance.Name)
		assert.Equal(t, "provider unavailable", resp.Session.Instances[0].Error)
		assert.Equal(t, "b", resp.Session.Instances[1].Instance.Name)
		assert.Equal(t, "", resp.Session.Instances[1].Error)
	})

	t.Run("empty session serializes an empty array, not null", func(t *testing.T) {
		resp := NewSessionResponse(&sablier.SessionState{})
		raw, err := json.Marshal(resp)
		assert.NilError(t, err)
		assert.Assert(t, string(raw) != "" && !containsSub(string(raw), `"instances":null`), "instances must be [] on the wire: %s", raw)
	})
}

// TestSessionResponseWireShape pins the historical wire contract of the
// blocking endpoint, previously produced by an ad-hoc map plus a custom
// SessionState marshaler: an object under "session" holding an ARRAY of
// {instance, error} entries and a "status" string. The generated OpenAPI
// schema is derived from these DTO types, so schema and wire can no longer
// drift apart.
func TestSessionResponseWireShape(t *testing.T) {
	s := &sablier.SessionState{
		Instances: map[string]sablier.InstanceInfoWithError{
			"web": {Instance: sablier.InstanceInfo{Name: "web", Status: sablier.InstanceStatusReady, CurrentReplicas: 1, DesiredReplicas: 1}},
		},
	}

	raw, err := json.Marshal(NewSessionResponse(s))
	assert.NilError(t, err)

	var m map[string]any
	assert.NilError(t, json.Unmarshal(raw, &m))

	session, ok := m["session"].(map[string]any)
	assert.Assert(t, ok, "top-level session object expected")
	assert.Equal(t, "ready", session["status"])

	instances, ok := session["instances"].([]any)
	assert.Assert(t, ok, "instances must be an array on the wire")
	entry := instances[0].(map[string]any)
	instance := entry["instance"].(map[string]any)
	assert.Equal(t, "web", instance["name"])
	_, hasError := entry["error"]
	assert.Assert(t, !hasError, "error must be omitted when empty")
}

func containsSub(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
