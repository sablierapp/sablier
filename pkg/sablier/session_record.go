package sablier

import (
	"encoding/json"
	"time"
)

// SessionRecordVersion is the current schema version of persisted session
// records. Bump it when the persisted shape changes, and handle the previous
// versions in SessionRecord.UnmarshalJSON.
const SessionRecordVersion = 2

// SessionRecord is the persisted form of one session entry: an explicit
// schema chosen field by field, not a serialization of the domain struct.
//
// It holds session STATE — the instance view served on the warm path (where
// no provider call happens) and the readiness semantics that apply for the
// session's lifetime. It deliberately excludes label CONFIGURATION (enabled,
// anti-affinity, scale profiles, the typed config bag): configuration is
// parsed fresh from the provider at every boundary and must not be frozen
// into sessions. Fields removed here disappear from warm-path API responses;
// that is intentional.
type SessionRecord struct {
	Version int `json:"v"`

	Name            string         `json:"name"`
	CurrentReplicas int32          `json:"currentReplicas"`
	DesiredReplicas int32          `json:"desiredReplicas"`
	Status          InstanceStatus `json:"status"`
	Message         string         `json:"message,omitempty"`
	Groups          []string       `json:"groups,omitempty"`

	// Provider identity and details, displayed by waiting-page themes.
	Provider   ProviderType            `json:"provider,omitempty"`
	Docker     *DockerContainerInfo    `json:"docker,omitempty"`
	Swarm      *SwarmServiceInfo       `json:"swarm,omitempty"`
	Kubernetes *KubernetesWorkloadInfo `json:"kubernetes,omitempty"`
	Podman     *PodmanContainerInfo    `json:"podman,omitempty"`

	// Readiness semantics that apply for the lifetime of the session:
	// IsReady() on the warm path is decided from these.
	ReadyAfter   time.Duration `json:"readyAfter,omitempty"`
	ReadyAt      *time.Time    `json:"readyAt,omitempty"`
	ReadyOnStart bool          `json:"readyOnStart,omitempty"`

	// Keep-warm window snapshot driving session-duration extension. Refreshed
	// from live labels on every not-ready inspect, so staleness is bounded by
	// the session lifecycle.
	RunningHours string `json:"runningHours,omitempty"`
	RunningDays  string `json:"runningDays,omitempty"`
}

// NewSessionRecord captures the session state of an instance view. Label
// configuration on the input is intentionally not persisted.
func NewSessionRecord(i InstanceInfo) SessionRecord {
	return SessionRecord{
		Version:         SessionRecordVersion,
		Name:            i.Name,
		CurrentReplicas: i.CurrentReplicas,
		DesiredReplicas: i.DesiredReplicas,
		Status:          i.Status,
		Message:         i.Message,
		Groups:          i.Groups,
		Provider:        i.Provider,
		Docker:          i.Docker,
		Swarm:           i.Swarm,
		Kubernetes:      i.Kubernetes,
		Podman:          i.Podman,
		ReadyAfter:      i.ReadyAfter,
		ReadyAt:         i.ReadyAt,
		ReadyOnStart:    i.ReadyOnStart,
		RunningHours:    i.RunningHours,
		RunningDays:     i.RunningDays,
	}
}

// ToInstanceInfo returns the instance view a stored session serves on the
// warm path. Label-configuration fields (Enabled, AntiAffinity, ScaleConfig,
// Config) are absent by design.
func (r SessionRecord) ToInstanceInfo() InstanceInfo {
	return InstanceInfo{
		Name:            r.Name,
		CurrentReplicas: r.CurrentReplicas,
		DesiredReplicas: r.DesiredReplicas,
		Status:          r.Status,
		Message:         r.Message,
		Groups:          r.Groups,
		Provider:        r.Provider,
		Docker:          r.Docker,
		Swarm:           r.Swarm,
		Kubernetes:      r.Kubernetes,
		Podman:          r.Podman,
		ReadyAfter:      r.ReadyAfter,
		ReadyAt:         r.ReadyAt,
		ReadyOnStart:    r.ReadyOnStart,
		RunningHours:    r.RunningHours,
		RunningDays:     r.RunningDays,
	}
}

// UnmarshalJSON decodes a session record, upgrading the two previous
// generations of persisted payloads:
//
//	v2 (current): the explicit record fields.
//	v1:           a {"v":1,"instance":{...}} envelope around the domain struct.
//	v0 (legacy):  a bare InstanceInfo document (releases before versioning).
//
// Anything that is valid JSON but none of these shapes decodes to a record
// with an empty Name, which stores use to skip foreign keys sharing their
// keyspace.
func (r *SessionRecord) UnmarshalJSON(b []byte) error {
	var probe struct {
		Version  int             `json:"v"`
		Instance json.RawMessage `json:"instance"`
	}
	if err := json.Unmarshal(b, &probe); err != nil {
		return err
	}

	switch probe.Version {
	case 1:
		var legacy InstanceInfo
		if err := json.Unmarshal(probe.Instance, &legacy); err != nil {
			return err
		}
		*r = NewSessionRecord(legacy)
		return nil
	case 0:
		var legacy InstanceInfo
		if err := json.Unmarshal(b, &legacy); err != nil {
			return err
		}
		*r = NewSessionRecord(legacy)
		return nil
	default:
		// Current version — and best-effort forward compatibility: a record
		// written by a NEWER schema decodes through the fields we know.
		type plain SessionRecord
		var p plain
		if err := json.Unmarshal(b, &p); err != nil {
			return err
		}
		*r = SessionRecord(p)
		return nil
	}
}
