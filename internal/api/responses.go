package api

import (
	"sort"

	"github.com/sablierapp/sablier/pkg/sablier"
)

// SessionResponse is the wire contract of the blocking strategy endpoint. It
// is built explicitly from the domain session by NewSessionResponse, so the
// handler returns exactly the type its OpenAPI annotation declares and the
// generated schema matches the bytes on the wire (the previous ad-hoc
// map+custom-marshaler pair had drifted: the published schema said `instances`
// was a map and omitted `status`, while the wire carried an array plus
// `status`).
type SessionResponse struct {
	Session SessionStateResponse `json:"session"`
}

// SessionStateResponse mirrors the historical wire shape of a session: an
// array of instance entries plus the aggregate session status ("ready" or
// "not-ready"). Entries are sorted by instance name; the order was previously
// map-iteration random and was never part of the contract.
type SessionStateResponse struct {
	Instances []InstanceEntryResponse `json:"instances"`
	Status    string                  `json:"status"`
}

// InstanceEntryResponse is one instance of the session, with the error that
// prevented it from starting, if any.
type InstanceEntryResponse struct {
	Instance sablier.InstanceInfo `json:"instance"`
	Error    string               `json:"error,omitempty"`
}

// NewSessionResponse maps a domain session to its wire representation.
func NewSessionResponse(s *sablier.SessionState) SessionResponse {
	instances := make([]InstanceEntryResponse, 0, len(s.Instances))
	for _, v := range s.Instances {
		entry := InstanceEntryResponse{Instance: v.Instance}
		if v.Error != nil {
			entry.Error = v.Error.Error()
		}
		instances = append(instances, entry)
	}
	sort.Slice(instances, func(i, j int) bool {
		return instances[i].Instance.Name < instances[j].Instance.Name
	})
	return SessionResponse{Session: SessionStateResponse{
		Instances: instances,
		Status:    s.Status(),
	}}
}

// ThemesResponse is the JSON body returned by the themes endpoint.
type ThemesResponse struct {
	Themes []string `json:"themes"`
}
