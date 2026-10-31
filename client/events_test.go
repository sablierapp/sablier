package client_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/sablierapp/sablier/client"
)

// eventServer serves body as an SSE stream and records the request.
func eventServer(t *testing.T, body string) (*client.Client, *http.Request) {
	t.Helper()
	got := &http.Request{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*got = *r.Clone(context.Background())
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)

	c, err := client.New(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	return c, got
}

func TestEvents(t *testing.T) {
	// The wire format gin writes: no space after the colon.
	c, got := eventServer(t, "event:created\ndata:{\"name\":\"web\",\"status\":\"starting\"}\n\n"+
		"event:removed\ndata:{\"name\":\"db\"}\n\n")

	stream, _, err := c.Events(context.Background(), client.StreamInstanceEventsParams{
		Group: ptr("demo"),
		Types: &[]string{"created", "removed"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = stream.Close() }()

	want := url.Values{"group": {"demo"}, "types": {"created", "removed"}}
	if got.URL.Path != "/api/events" || got.URL.Query().Encode() != want.Encode() {
		t.Errorf("request = %s?%s, want /api/events?%s", got.URL.Path, got.URL.RawQuery, want.Encode())
	}
	if accept := got.Header.Get("Accept"); accept != "text/event-stream" {
		t.Errorf("Accept = %q, want text/event-stream", accept)
	}

	first, err := stream.Next()
	if err != nil {
		t.Fatal(err)
	}
	if first.Type != "created" || *first.Instance.Name != "web" || *first.Instance.Status != "starting" {
		t.Errorf("first event = %s %+v, want created web starting", first.Type, first.Instance)
	}
	second, err := stream.Next()
	if err != nil {
		t.Fatal(err)
	}
	if second.Type != "removed" || *second.Instance.Name != "db" {
		t.Errorf("second event = %s %s, want removed db", second.Type, *second.Instance.Name)
	}
	if _, err := stream.Next(); !errors.Is(err, io.EOF) {
		t.Errorf("Next() after the stream ended = %v, want io.EOF", err)
	}
}

func TestEventsParsing(t *testing.T) {
	// Spaces after colons, comments, unknown fields, CRLF line endings and
	// data split over several lines are all valid SSE.
	c, _ := eventServer(t, ": keep-alive\n\n"+
		"id: 1\r\nevent: started\r\ndata: {\"name\":\r\ndata: \"web\"}\r\n\r\n")

	stream, _, err := c.Events(context.Background(), client.StreamInstanceEventsParams{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = stream.Close() }()

	event, err := stream.Next()
	if err != nil {
		t.Fatal(err)
	}
	if event.Type != "started" || *event.Instance.Name != "web" {
		t.Errorf("event = %s %+v, want started web", event.Type, event.Instance)
	}
}

func TestEventsErrorResponse(t *testing.T) {
	c, _ := sablier(t, http.StatusBadRequest, "application/problem+json", "",
		`{"title":"Validation Failed","status":400,"detail":"'names' and 'group' query parameters are both set"}`)

	stream, resp, err := c.Events(context.Background(), client.StreamInstanceEventsParams{Names: &[]string{"web"}, Group: ptr("demo")})

	// A type assertion, as plugins under Yaegi do: the error is never wrapped.
	errResp, ok := err.(*client.ErrorResponse)
	if !ok {
		t.Fatalf("err = %v, want a *client.ErrorResponse", err)
	}
	if stream != nil {
		t.Error("stream must be nil for an error response")
	}
	if resp == nil || resp.StatusCode != http.StatusBadRequest {
		t.Errorf("resp = %v, want the 400 response", resp)
	}
	if p := errResp.Problem; p == nil || p.Title != "Validation Failed" {
		t.Errorf("problem = %+v, want Validation Failed", p)
	}
}

func TestEventsStopsOnCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.(http.Flusher).Flush()
		<-r.Context().Done() // hold the stream open, as Sablier does
	}))
	t.Cleanup(srv.Close)
	c, err := client.New(srv.URL)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	stream, _, err := c.Events(ctx, client.StreamInstanceEventsParams{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = stream.Close() }()

	done := make(chan error, 1)
	go func() {
		_, err := stream.Next()
		done <- err
	}()
	cancel()

	select {
	case err := <-done:
		if err == nil {
			t.Error("Next() returned no error after cancel")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Next() still blocked 5s after cancel")
	}
}

func TestEventsCutMidEvent(t *testing.T) {
	// An event is only complete once its blank line arrives.
	for _, body := range []string{"event:created\ndata:{}", "event:created\ndata:{}\n"} {
		c, _ := eventServer(t, body)
		stream, _, err := c.Events(context.Background(), client.StreamInstanceEventsParams{})
		if err != nil {
			t.Fatal(err)
		}

		if _, err := stream.Next(); !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Errorf("Next() on %q = %v, want io.ErrUnexpectedEOF", body, err)
		}
		_ = stream.Close()
	}
}
