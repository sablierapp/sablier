package api

import "github.com/sablierapp/sablier/pkg/sablier"

// SessionResponse is the JSON body returned by the blocking strategy endpoint.
type SessionResponse struct {
	Session *sablier.SessionState `json:"session"`
}

// ThemesResponse is the JSON body returned by the themes endpoint.
type ThemesResponse struct {
	Themes []string `json:"themes"`
}
