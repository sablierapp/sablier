package client

import (
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
)

const problemMediaType = "application/problem+json"

// Problem is an RFC 7807 problem detail, the body of Sablier's error
// responses.
//
// It is written by hand rather than generated from rfc7807.Problem: the
// generated type decodes its extension members with custom JSON methods,
// which crash Yaegi, the interpreter Traefik runs plugins with.
type Problem struct {
	Type     string `json:"type,omitempty"`
	Title    string `json:"title,omitempty"`
	Status   int    `json:"status,omitempty"`
	Detail   string `json:"detail,omitempty"`
	Instance string `json:"instance,omitempty"`
	// Extensions holds the members a problem type adds, such as the
	// availableGroups of a group that was not found.
	Extensions map[string]json.RawMessage `json:"-"`
}

// ErrorResponse is the error returned when Sablier answers with a status
// other than 2xx, such as a validation problem or a blocking timeout.
//
// It is never wrapped, so code running under Yaegi, such as a Traefik plugin,
// can check for it with a type assertion: errors.As panics there when its
// target is an interpreted type.
type ErrorResponse struct {
	Response *Response
	// Problem is the problem detail of the answer, or nil when it has none.
	Problem *Problem
}

func (e *ErrorResponse) Error() string {
	if e.Problem != nil && e.Problem.Detail != "" {
		return fmt.Sprintf("sablier answered %d: %s", e.Response.StatusCode, e.Problem.Detail)
	}
	return fmt.Sprintf("sablier answered %d", e.Response.StatusCode)
}

// checkResponse returns an *ErrorResponse when resp is not a 2xx answer.
func checkResponse(resp *Response, body []byte) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return &ErrorResponse{Response: resp, Problem: decodeProblem(resp.Header, body)}
}

// decodeProblem decodes body as a problem detail. It returns nil when body is
// not one.
func decodeProblem(header http.Header, body []byte) *Problem {
	mediaType, _, err := mime.ParseMediaType(header.Get("Content-Type"))
	if err != nil || mediaType != problemMediaType {
		return nil
	}
	var problem Problem
	if err := json.Unmarshal(body, &problem); err != nil {
		return nil
	}
	var members map[string]json.RawMessage
	if err := json.Unmarshal(body, &members); err == nil {
		for _, name := range []string{"type", "title", "status", "detail", "instance"} {
			delete(members, name)
		}
		if len(members) > 0 {
			problem.Extensions = members
		}
	}
	return &problem
}
