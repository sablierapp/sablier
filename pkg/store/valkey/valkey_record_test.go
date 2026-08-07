package valkey

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/sablierapp/sablier/pkg/sablier"
	"gotest.tools/v3/assert"
)

// TestValKeySessionRecordMigration pins the persistence schema: new entries
// are written as versioned session records, and entries written by older
// versions (bare InstanceInfo payloads) are transparently upgraded on read,
// so live sessions survive a Sablier upgrade without being dropped.
func TestValKeySessionRecordMigration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	ctx := t.Context()
	vk := setupValKey(t)

	t.Parallel()
	t.Run("legacy payload is upgraded on read", func(t *testing.T) {
		legacy := `{"name":"legacy-instance","currentReplicas":1,"desiredReplicas":1,"status":"ready"}`
		err := vk.Client.Do(ctx, vk.Client.B().Set().Key("legacy-instance").Value(legacy).Ex(time.Minute).Build()).Error()
		assert.NilError(t, err)

		info, err := vk.Get(ctx, "legacy-instance")
		assert.NilError(t, err)
		assert.Equal(t, "legacy-instance", info.Name)
		assert.Equal(t, sablier.InstanceStatusReady, info.Status)

		// The legacy entry must also be visible to Range (the metrics path).
		seen := false
		err = vk.Range(ctx, func(i sablier.InstanceInfo, _ time.Time) {
			if i.Name == "legacy-instance" {
				seen = true
			}
		})
		assert.NilError(t, err)
		assert.Assert(t, seen, "legacy entry must be enumerated by Range")
	})

	t.Run("new entries carry the schema version", func(t *testing.T) {
		err := vk.Put(ctx, sablier.InstanceInfo{Name: "versioned-instance", Status: sablier.InstanceStatusReady}, time.Minute)
		assert.NilError(t, err)

		raw, err := vk.Client.Do(ctx, vk.Client.B().Get().Key("versioned-instance").Build()).AsBytes()
		assert.NilError(t, err)
		var m map[string]any
		assert.NilError(t, json.Unmarshal(raw, &m))
		assert.Equal(t, float64(sablier.SessionRecordVersion), m["v"])

		info, err := vk.Get(ctx, "versioned-instance")
		assert.NilError(t, err)
		assert.Equal(t, "versioned-instance", info.Name)
	})
}
