package inmemory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/sablierapp/sablier/pkg/sablier"
	"gotest.tools/v3/assert"
)

// TestInMemorySnapshotIsVersioned pins the persisted snapshot schema: entries
// written to state.json carry the session-record version, and a snapshot in
// the legacy shape (bare InstanceInfo values, as written by older versions)
// still loads. The load path is additionally covered end to end by the
// sabliercmd storage tests, whose fixtures use the legacy shape.
func TestInMemorySnapshotIsVersioned(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("snapshot carries the schema version", func(t *testing.T) {
		t.Parallel()
		s := NewInMemory()
		assert.NilError(t, s.Put(ctx, sablier.InstanceInfo{Name: "web", Status: sablier.InstanceStatusReady}, time.Minute))

		raw, err := s.(*InMemory).MarshalJSON()
		assert.NilError(t, err)
		versionTag := fmt.Sprintf(`"v":%d`, sablier.SessionRecordVersion)
		assert.Assert(t, strings.Contains(string(raw), versionTag), "snapshot entries must be versioned: %s", raw)
	})

	t.Run("legacy snapshot loads and upgrades", func(t *testing.T) {
		t.Parallel()
		legacy := map[string]any{
			"web": map[string]any{
				"value":     map[string]any{"name": "web", "currentReplicas": 1, "desiredReplicas": 1, "status": "ready"},
				"expiresAt": time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
			},
		}
		raw, err := json.Marshal(legacy)
		assert.NilError(t, err)

		s := NewInMemory()
		assert.NilError(t, s.(*InMemory).UnmarshalJSON(raw))

		info, err := s.Get(ctx, "web")
		assert.NilError(t, err)
		assert.Equal(t, "web", info.Name)
		assert.Equal(t, sablier.InstanceStatusReady, info.Status)
	})
}
