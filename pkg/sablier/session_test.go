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

func TestSessionState_MarshalJSON(t *testing.T) {
	s := &sablier.SessionState{
		Instances: map[string]sablier.InstanceInfoWithError{
			"a": {Instance: sablier.InstanceInfo{Name: "a", Status: sablier.InstanceStatusReady}},
		},
	}

	b, err := s.MarshalJSON()
	assert.NilError(t, err)
	assert.Assert(t, contains(string(b), `"status":"ready"`))
}

func TestSessionState_MarshalJSON_ErrorField(t *testing.T) {
	s := &sablier.SessionState{
		Instances: map[string]sablier.InstanceInfoWithError{
			"a": {Instance: sablier.InstanceInfo{Name: "a"}, Error: errors.New("provider unavailable")},
		},
	}

	b, err := s.MarshalJSON()
	assert.NilError(t, err)
	assert.Assert(t, contains(string(b), `"error":"provider unavailable"`))
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
