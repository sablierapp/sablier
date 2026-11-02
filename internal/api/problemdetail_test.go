package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/sablierapp/sablier/pkg/sablier"
	"gotest.tools/v3/assert"
)

func TestProblemTimeout_NoInstances(t *testing.T) {
	pb := ProblemTimeout(sablier.ErrTimeout{Duration: 5 * time.Second})
	assert.Equal(t, pb.Status, http.StatusGatewayTimeout)
	assert.Equal(t, pb.Detail, "session was not ready after 5s")
}

func TestProblemTimeout_WithInstances(t *testing.T) {
	e := sablier.ErrTimeout{
		Duration: 30 * time.Second,
		Instances: []sablier.InstanceInfoWithError{
			{Instance: sablier.InstanceInfo{
				Name:    "nextcloud",
				Status:  sablier.InstanceStatusNotReady,
				Message: `paused while group "streaming" is active (anti-affinity)`,
			}},
		},
	}

	pb := ProblemTimeout(e)
	assert.Equal(t, pb.Status, http.StatusGatewayTimeout)
	assert.Assert(t, strings.Contains(pb.Detail, "session was not ready after 30s"), pb.Detail)
	assert.Assert(t, strings.Contains(pb.Detail, "nextcloud"), pb.Detail)
	assert.Assert(t, strings.Contains(pb.Detail, "streaming"), pb.Detail)

	// The structured per-instance breakdown is exposed as an extension member.
	b, err := json.Marshal(pb)
	assert.NilError(t, err)
	assert.Assert(t, strings.Contains(string(b), `"instances"`), string(b))
	assert.Assert(t, strings.Contains(string(b), `"nextcloud"`), string(b))
}

func TestProblemGroupNotFound_DoesNotLeakGroups(t *testing.T) {
	pb := ProblemGroupNotFound(sablier.ErrGroupNotFound{
		Group:           "app-typo",
		AvailableGroups: []string{"app-a", "app-b"},
	})
	assert.Equal(t, pb.Status, http.StatusNotFound)

	body, err := json.Marshal(pb)
	assert.NilError(t, err)
	assert.Assert(t, !strings.Contains(string(body), "availableGroups"), "body: %s", body)
	assert.Assert(t, !strings.Contains(string(body), "app-a"), "body: %s", body)
	assert.Assert(t, !strings.Contains(string(body), "app-b"), "body: %s", body)
	assert.Assert(t, strings.Contains(string(body), "app-typo"), "the requested group is the caller's own input: %s", body)
}
