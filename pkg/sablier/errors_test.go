package sablier_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/sablierapp/sablier/pkg/sablier"
	"gotest.tools/v3/assert"
)

func TestErrGroupNotFound_Error(t *testing.T) {
	err := sablier.ErrGroupNotFound{Group: "missing"}
	assert.Equal(t, err.Error(), "group missing not found")
}

func TestErrRequestBinding_Error(t *testing.T) {
	err := sablier.ErrRequestBinding{Err: errors.New("invalid payload")}
	assert.Equal(t, err.Error(), "invalid payload")
}

func TestErrTimeout_Error(t *testing.T) {
	err := sablier.ErrTimeout{Duration: 3 * time.Second}
	assert.Equal(t, err.Error(), "timeout after 3s")
}

func TestErrTimeout_ErrorWithInstances(t *testing.T) {
	err := sablier.ErrTimeout{
		Duration: 30 * time.Second,
		Instances: []sablier.InstanceInfoWithError{
			{Instance: sablier.InstanceInfo{
				Name:    "nextcloud",
				Status:  sablier.InstanceStatusNotReady,
				Message: `paused while group "streaming" is active (anti-affinity)`,
			}},
			{Instance: sablier.InstanceInfo{Name: "db", Status: sablier.InstanceStatusStarting}, Error: errors.New("boom")},
		},
	}

	msg := err.Error()
	assert.Assert(t, strings.Contains(msg, "timeout after 30s"), msg)
	assert.Assert(t, strings.Contains(msg, `nextcloud: not-ready (paused while group "streaming" is active (anti-affinity))`), msg)
	assert.Assert(t, strings.Contains(msg, "db: boom"), msg)

	reasons := err.InstanceReasons()
	assert.Equal(t, len(reasons), 2)
}
