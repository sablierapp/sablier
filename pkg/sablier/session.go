package sablier

import (
	"encoding/json"
	"errors"
	"fmt"
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

// InstanceErrors returns a joined error if any instance has a non-nil error.
func (s *SessionState) InstanceErrors() error {
	var errs []error
	for name, v := range s.Instances {
		if v.Error != nil {
			errs = append(errs, fmt.Errorf("%s: %w", name, v.Error))
		}
	}
	return errors.Join(errs...)
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
