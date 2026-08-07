package sablier

import (
	"encoding/json"
	"testing"

	"gotest.tools/v3/assert"
)

func TestSessionRecordRoundTrip(t *testing.T) {
	rec := NewSessionRecord(InstanceInfo{Name: "web", Status: InstanceStatusReady, CurrentReplicas: 1, DesiredReplicas: 1})
	assert.Equal(t, SessionRecordVersion, rec.Version)

	b, err := json.Marshal(rec)
	assert.NilError(t, err)
	assert.Assert(t, containsSub(string(b), `"v":1`), "persisted record must carry its schema version: %s", b)

	var got SessionRecord
	assert.NilError(t, json.Unmarshal(b, &got))
	assert.DeepEqual(t, rec, got)
}

// TestSessionRecordLegacyUpgrade pins the migration path: records persisted
// before versioning were a bare InstanceInfo document; they must decode into
// a current-version record with every field intact, so live sessions survive
// the upgrade (both in valkey and in the state.json snapshot).
func TestSessionRecordLegacyUpgrade(t *testing.T) {
	legacy := `{"name":"web","currentReplicas":1,"desiredReplicas":2,"status":"ready","enabled":"true","groups":["default"],"runningHours":"09:00-18:00"}`

	var got SessionRecord
	assert.NilError(t, json.Unmarshal([]byte(legacy), &got))

	assert.Equal(t, SessionRecordVersion, got.Version)
	assert.Equal(t, "web", got.Instance.Name)
	assert.Equal(t, int32(1), got.Instance.CurrentReplicas)
	assert.Equal(t, int32(2), got.Instance.DesiredReplicas)
	assert.Equal(t, InstanceStatusReady, got.Instance.Status)
	assert.Equal(t, "true", got.Instance.Enabled)
	assert.DeepEqual(t, []string{"default"}, got.Instance.Groups)
	assert.Equal(t, "09:00-18:00", got.Instance.RunningHours)
}

func TestSessionRecordForeignPayloads(t *testing.T) {
	t.Run("invalid JSON errors", func(t *testing.T) {
		var got SessionRecord
		assert.Assert(t, json.Unmarshal([]byte(`not json`), &got) != nil)
	})
	t.Run("foreign object decodes to a zero record", func(t *testing.T) {
		// Stores sharing their keyspace skip entries whose Instance.Name does
		// not match the key; a foreign JSON object must therefore decode to a
		// record with an empty name rather than erroring the enumeration.
		var got SessionRecord
		assert.NilError(t, json.Unmarshal([]byte(`{"foo":"bar"}`), &got))
		assert.Equal(t, "", got.Instance.Name)
	})
}

func containsSub(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
