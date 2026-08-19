package sablier

import (
	"encoding/json"
	"testing"
	"time"

	"gotest.tools/v3/assert"
)

func TestSessionRecordRoundTrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	info := InstanceInfo{
		Name:            "web",
		CurrentReplicas: 1,
		DesiredReplicas: 2,
		Status:          InstanceStatusReady,
		Groups:          []string{"default"},
		Provider:        ProviderDocker,
		Docker:          &DockerContainerInfo{ID: "abc", Image: "img"},
		ReadyAfter:      30 * time.Second,
		ReadyAt:         &now,
		ReadyOnStart:    true,
		RunningHours:    "09:00-18:00",
		RunningDays:     "Mon,Tue",
	}

	rec := NewSessionRecord(info)
	assert.Equal(t, SessionRecordVersion, rec.Version)

	b, err := json.Marshal(rec)
	assert.NilError(t, err)

	var got SessionRecord
	assert.NilError(t, json.Unmarshal(b, &got))
	assert.DeepEqual(t, rec, got)
	assert.DeepEqual(t, info, got.ToInstanceInfo())
}

// TestSessionRecordDropsConfiguration pins the schema decision: label
// configuration is not session state and must not be persisted or served on
// the warm path, while the readiness semantics (ReadyAfter/ReadyAt/
// ReadyOnStart) that IsReady depends on are preserved.
func TestSessionRecordDropsConfiguration(t *testing.T) {
	now := time.Now()
	info := InstanceInfo{
		Name:         "web",
		Status:       InstanceStatusReady,
		Enabled:      "true",
		AntiAffinity: []string{"streaming"},
		ScaleConfig:  &ScaleConfig{Idle: ResourceProfile{Replicas: 1}},
		Config:       &InstanceConfig{Enabled: true, Groups: []string{"default"}},
		ReadyAfter:   time.Second,
		ReadyAt:      &now,
		ReadyOnStart: true,
	}

	out := NewSessionRecord(info).ToInstanceInfo()

	// Configuration: gone.
	assert.Equal(t, "", out.Enabled)
	assert.Assert(t, out.AntiAffinity == nil)
	assert.Assert(t, out.ScaleConfig == nil)
	assert.Assert(t, out.Config == nil)

	// Readiness semantics: intact, so the warm path answers IsReady correctly.
	assert.Equal(t, time.Second, out.ReadyAfter)
	assert.Assert(t, out.ReadyAt != nil)
	assert.Assert(t, out.ReadyOnStart)
	assert.Assert(t, out.IsReady())
}

// TestSessionRecordUpgrades pins the migration paths for both previous
// generations of persisted payloads.
func TestSessionRecordUpgrades(t *testing.T) {
	t.Run("v0: bare InstanceInfo document", func(t *testing.T) {
		legacy := `{"name":"web","currentReplicas":1,"desiredReplicas":2,"status":"ready","enabled":"true","groups":["default"],"runningHours":"09:00-18:00"}`

		var got SessionRecord
		assert.NilError(t, json.Unmarshal([]byte(legacy), &got))

		assert.Equal(t, SessionRecordVersion, got.Version)
		assert.Equal(t, "web", got.Name)
		assert.Equal(t, int32(2), got.DesiredReplicas)
		assert.Equal(t, InstanceStatusReady, got.Status)
		assert.DeepEqual(t, []string{"default"}, got.Groups)
		assert.Equal(t, "09:00-18:00", got.RunningHours)
	})

	t.Run("v1: envelope around the domain struct", func(t *testing.T) {
		v1 := `{"v":1,"instance":{"name":"web","currentReplicas":1,"desiredReplicas":1,"status":"ready","enabled":"true"}}`

		var got SessionRecord
		assert.NilError(t, json.Unmarshal([]byte(v1), &got))

		assert.Equal(t, SessionRecordVersion, got.Version)
		assert.Equal(t, "web", got.Name)
		assert.Equal(t, InstanceStatusReady, got.Status)
	})
}

func TestSessionRecordForeignPayloads(t *testing.T) {
	t.Run("invalid JSON errors", func(t *testing.T) {
		var got SessionRecord
		assert.Assert(t, json.Unmarshal([]byte(`not json`), &got) != nil)
	})
	t.Run("foreign object decodes to a zero record", func(t *testing.T) {
		// Stores sharing their keyspace skip entries whose Name does not match
		// the key; a foreign JSON object must decode to a record with an empty
		// name rather than erroring the enumeration.
		var got SessionRecord
		assert.NilError(t, json.Unmarshal([]byte(`{"foo":"bar"}`), &got))
		assert.Equal(t, "", got.Name)
	})
}
