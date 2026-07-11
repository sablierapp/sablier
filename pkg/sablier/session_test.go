package sablier_test

import (
	"errors"
	"testing"

	"github.com/sablierapp/sablier/pkg/sablier"
	"gotest.tools/v3/assert"
)

func TestSessionState_IsReady_NilInstances(t *testing.T) {
	s := &sablier.SessionState{}
	assert.Assert(t, s.IsReady())
	assert.Assert(t, s.Instances != nil)
}

func TestSessionState_IsReady_FalseWhenInstanceHasError(t *testing.T) {
	s := &sablier.SessionState{
		Instances: map[string]sablier.InstanceInfoWithError{
			"a": {
				Instance: sablier.InstanceInfo{Status: sablier.InstanceStatusReady},
				Error:    errors.New("boom"),
			},
		},
	}
	assert.Assert(t, !s.IsReady())
}

func TestSessionState_IsReady_FalseWhenInstanceNotReady(t *testing.T) {
	s := &sablier.SessionState{
		Instances: map[string]sablier.InstanceInfoWithError{
			"a": {
				Instance: sablier.InstanceInfo{Status: sablier.InstanceStatusStarting},
			},
		},
	}
	assert.Assert(t, !s.IsReady())
}

// A completed one-shot is a distinct terminal state from ready: it is not
// running and serving traffic, so a session that contains one is not ready.
// This is why a one-shot must never be a labeled member of a blocking group.
func TestSessionState_IsReady_FalseWhenInstanceCompleted(t *testing.T) {
	s := &sablier.SessionState{
		Instances: map[string]sablier.InstanceInfoWithError{
			"a": {
				Instance: sablier.InstanceInfo{Status: sablier.InstanceStatusCompleted},
			},
		},
	}
	assert.Assert(t, !s.IsReady())
}

func TestInstanceInfo_IsReady_CompletedIsNotReady(t *testing.T) {
	assert.Assert(t, !sablier.InstanceInfo{Status: sablier.InstanceStatusCompleted}.IsReady())
	assert.Assert(t, sablier.InstanceInfo{Status: sablier.InstanceStatusReady}.IsReady())
}

func TestSessionState_InstanceErrors(t *testing.T) {
	s := &sablier.SessionState{
		Instances: map[string]sablier.InstanceInfoWithError{
			"a": {
				Instance: sablier.InstanceInfo{Status: sablier.InstanceStatusReady},
				Error:    errors.New("first"),
			},
			"b": {
				Instance: sablier.InstanceInfo{Status: sablier.InstanceStatusReady},
				Error:    errors.New("second"),
			},
		},
	}

	err := s.InstanceErrors()
	assert.Assert(t, err != nil)
	assert.Assert(t, contains(err.Error(), "a: first") || contains(err.Error(), "b: second"))
}

func TestSessionState_Status(t *testing.T) {
	ready := &sablier.SessionState{
		Instances: map[string]sablier.InstanceInfoWithError{
			"a": {Instance: sablier.InstanceInfo{Status: sablier.InstanceStatusReady}},
		},
	}
	notReady := &sablier.SessionState{
		Instances: map[string]sablier.InstanceInfoWithError{
			"a": {Instance: sablier.InstanceInfo{Status: sablier.InstanceStatusStarting}},
		},
	}

	assert.Equal(t, ready.Status(), "ready")
	assert.Equal(t, notReady.Status(), "not-ready")
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && (indexOf(s, sub) >= 0))
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
