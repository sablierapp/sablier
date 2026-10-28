package client

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Path of the events endpoint, checked against the spec by TestPathsMatchSpec.
const pathStreamInstanceEvents = "/api/events"

// Event is an instance lifecycle event.
type Event struct {
	// Type is created, updated, removed, started or stopped.
	Type string
	// Instance is the instance as Sablier saw it when the event happened.
	// Removal events may only carry its name.
	Instance SablierInstanceInfo
}

// StatusError is returned when Sablier answers with something other than the
// expected response, such as a validation problem.
type StatusError struct {
	Response *Response
}

func (e *StatusError) Error() string {
	if p := e.Response.Problem(); p != nil && p.Detail != nil {
		return fmt.Sprintf("sablier answered %d: %s", e.Response.StatusCode, *p.Detail)
	}
	return fmt.Sprintf("sablier answered %d", e.Response.StatusCode)
}

// EventStream reads instance lifecycle events from Sablier. Call Next to
// read events and Close when done. It is not safe for concurrent use.
type EventStream struct {
	body   io.ReadCloser
	reader *bufio.Reader
}

// Events subscribes to instance lifecycle events, filtered by params. The
// stream stays open until ctx is cancelled, Close is called, or Sablier ends
// it. A timeout on the HTTP client set with WithHTTPClient also ends it, so
// use a client without one for long-lived subscriptions.
//
// When Sablier answers with an error, Events returns a *StatusError.
func (c *Client) Events(ctx context.Context, params StreamInstanceEventsParams) (*EventStream, error) {
	resp, err := c.do(ctx, pathStreamInstanceEvents, params, http.Header{"Accept": {"text/event-stream"}})
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		defer func() { _ = resp.Body.Close() }()
		r, err := readResponse(resp)
		if err != nil {
			return nil, err
		}
		return nil, &StatusError{Response: r}
	}
	return &EventStream{body: resp.Body, reader: bufio.NewReader(resp.Body)}, nil
}

// Next blocks until the next event. It returns io.EOF when Sablier ends the
// stream, and the context error once the context passed to Events is done.
func (s *EventStream) Next() (Event, error) {
	var event Event
	var data []string
	for {
		line, err := s.reader.ReadString('\n')
		if err != nil {
			if err == io.EOF && line == "" && len(data) == 0 {
				return Event{}, io.EOF
			}
			if err != io.EOF {
				return Event{}, err
			}
			// A final event without its trailing blank line is incomplete.
			return Event{}, io.ErrUnexpectedEOF
		}
		line = strings.TrimRight(line, "\r\n")

		if line == "" {
			// A blank line dispatches the event; skip blocks without data,
			// such as keep-alive comments.
			if len(data) == 0 {
				event = Event{}
				continue
			}
			if err := json.Unmarshal([]byte(strings.Join(data, "\n")), &event.Instance); err != nil {
				return Event{}, fmt.Errorf("decode %s event: %w", event.Type, err)
			}
			return event, nil
		}

		field, value, _ := strings.Cut(line, ":")
		value = strings.TrimPrefix(value, " ")
		switch field {
		case "event":
			event.Type = value
		case "data":
			data = append(data, value)
		}
		// Comments (empty field) and other fields such as id and retry are
		// not used by Sablier.
	}
}

// Close ends the subscription.
func (s *EventStream) Close() error {
	return s.body.Close()
}
