package sablier

import (
	"errors"
	"fmt"
	"sort"
)

type SessionState struct {
	Instances map[string]InstanceInfoWithError `json:"instances"`
}

// IsReady reports whether every gating instance is ready and error-free.
// Optional (best-effort) instances never gate: a session whose only pending
// or failing members are optional is ready.
func (s *SessionState) IsReady() bool {
	if s.Instances == nil {
		s.Instances = map[string]InstanceInfoWithError{}
	}

	for _, v := range s.Instances {
		if v.Optional {
			continue
		}
		if v.Error != nil || !v.Instance.IsReady() {
			return false
		}
	}

	return true
}

// NotReadyInstances returns the instances that are not ready yet (or errored),
// sorted by name for stable output. Ready instances are omitted, as are
// optional instances: they never gate the session, so they cannot be the
// reason it did not become ready. It is used to explain why a session did not
// become ready within a blocking timeout.
func (s *SessionState) NotReadyInstances() []InstanceInfoWithError {
	var out []InstanceInfoWithError
	for _, v := range s.Instances {
		if v.Optional {
			continue
		}
		if v.Error != nil || !v.Instance.IsReady() {
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Instance.Name < out[j].Instance.Name
	})
	return out
}

// InstanceErrors returns a joined error if any gating instance has a non-nil
// error. Errors of optional instances are ignored by design: best-effort
// members must not fail a blocking request.
func (s *SessionState) InstanceErrors() error {
	var errs []error
	for name, v := range s.Instances {
		if v.Optional {
			continue
		}
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

