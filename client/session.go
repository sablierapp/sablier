package client

import "net/http"

// SessionStatusHeader is the response header in which the strategy endpoints
// report whether the session is ready.
const SessionStatusHeader = "X-Sablier-Session-Status"

// Values of SessionStatusHeader.
const (
	SessionStatusReady    = "ready"
	SessionStatusNotReady = "not-ready"
)

// IsReady reports whether a strategy response says the session is ready.
// Error responses carry no session status and are never ready.
func IsReady(resp *http.Response) bool {
	return resp != nil && resp.Header.Get(SessionStatusHeader) == SessionStatusReady
}
