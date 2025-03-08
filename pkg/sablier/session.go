package sablier

import (
	"encoding/json"
	"maps"
)

type SessionState struct {
	Instances map[string]InstanceInfoWithError `json:"instances"`
}

func (s *SessionState) IsReady() bool {
	if s.Instances == nil {
		s.Instances = map[string]InstanceInfoWithError{}
	}

	for _, v := range s.Instances {
		if v.Error != nil || v.Instance.Status != InstanceStatusReady {
			return false
		}
	}

	return true
}

func (s *SessionState) Status() string {
	if s.IsReady() {
		return "ready"
	}

	return "not-ready"
}

func (s *SessionState) MarshalJSON() ([]byte, error) {
	instances := maps.Values(s.Instances)

	return json.Marshal(map[string]any{
		"instances": instances,
		"status":    s.Status(),
	})
}
