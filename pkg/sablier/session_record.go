package sablier

import "encoding/json"

// SessionRecordVersion is the current schema version of persisted session
// records. Bump it when the persisted shape changes, and handle the previous
// versions in SessionRecord.UnmarshalJSON.
const SessionRecordVersion = 1

// SessionRecord is the persisted form of one session entry, used by every
// Store implementation. It exists so the storage schema is explicit and
// versioned instead of being whatever the domain struct happens to marshal
// as: Version gates future schema evolution, and UnmarshalJSON transparently
// upgrades records written before versioning existed (a bare InstanceInfo
// document), so live state survives the upgrade.
type SessionRecord struct {
	Version  int          `json:"v"`
	Instance InstanceInfo `json:"instance"`
}

// NewSessionRecord wraps an instance snapshot in the current schema version.
func NewSessionRecord(instance InstanceInfo) SessionRecord {
	return SessionRecord{Version: SessionRecordVersion, Instance: instance}
}

// UnmarshalJSON decodes a session record, upgrading legacy payloads: records
// written before versioning were a bare InstanceInfo document (no "v" key).
// Anything that is valid JSON but neither shape decodes to a zero record,
// which callers detect through the empty Instance.Name (mirroring the
// pre-versioning tolerance for foreign keys sharing the keyspace).
func (r *SessionRecord) UnmarshalJSON(b []byte) error {
	type plain SessionRecord
	var p plain
	if err := json.Unmarshal(b, &p); err != nil {
		return err
	}
	if p.Version == 0 {
		var legacy InstanceInfo
		if err := json.Unmarshal(b, &legacy); err != nil {
			return err
		}
		*r = SessionRecord{Version: SessionRecordVersion, Instance: legacy}
		return nil
	}
	*r = SessionRecord(p)
	return nil
}
