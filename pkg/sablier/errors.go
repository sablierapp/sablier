package sablier

import (
	"fmt"
	"strings"
	"time"
)

type ErrGroupNotFound struct {
	Group           string
	AvailableGroups []string
}

func (g ErrGroupNotFound) Error() string {
	return fmt.Sprintf("group %s not found", g.Group)
}

type ErrRequestBinding struct {
	Err error
}

func (e ErrRequestBinding) Error() string {
	return e.Err.Error()
}

type ErrTimeout struct {
	Duration time.Duration
	// Instances holds the not-ready instances captured from the last readiness
	// check before the timeout fired, so callers can explain what was still
	// pending (and why, e.g. an anti-affinity hold). It may be empty.
	Instances []InstanceInfoWithError
}

func (e ErrTimeout) Error() string {
	if len(e.Instances) == 0 {
		return fmt.Sprintf("timeout after %s", e.Duration)
	}
	return fmt.Sprintf("timeout after %s: %s", e.Duration, strings.Join(e.InstanceReasons(), "; "))
}

// InstanceReasons returns one human-readable line per not-ready instance,
// e.g. `nextcloud: not-ready (paused while group "streaming" is active ...)`.
// The instance's Message (if any) or Error is included as the reason.
func (e ErrTimeout) InstanceReasons() []string {
	reasons := make([]string, 0, len(e.Instances))
	for _, i := range e.Instances {
		switch {
		case i.Error != nil:
			reasons = append(reasons, fmt.Sprintf("%s: %s", i.Instance.Name, i.Error.Error()))
		case i.Instance.Message != "":
			reasons = append(reasons, fmt.Sprintf("%s: %s (%s)", i.Instance.Name, i.Instance.Status, i.Instance.Message))
		default:
			reasons = append(reasons, fmt.Sprintf("%s: %s", i.Instance.Name, i.Instance.Status))
		}
	}
	return reasons
}

type ErrInstanceNotManaged struct {
	Name string
}

func (e ErrInstanceNotManaged) Error() string {
	return fmt.Sprintf("instance %s is not managed by sablier", e.Name)
}
