package client_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/sablierapp/sablier/client"
)

// request is what the fake Sablier server received.
type request struct {
	path      string
	query     url.Values
	userAgent string
}

// sablier starts a server that records the last request and answers with the
// given status, content type, session status header and body.
func sablier(t *testing.T, status int, contentType, sessionStatus, body string, opts ...client.Option) (*client.Client, *request) {
	t.Helper()
	got := &request{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.path = r.URL.Path
		got.query = r.URL.Query()
		got.userAgent = r.UserAgent()
		w.Header().Set("Content-Type", contentType)
		if sessionStatus != "" {
			w.Header().Set(client.SessionStatusHeader, sessionStatus)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	c, err := client.New(srv.URL, opts...)
	if err != nil {
		t.Fatal(err)
	}
	return c, got
}

func ptr[T any](v T) *T { return &v }

func TestStartBlocking(t *testing.T) {
	c, got := sablier(t, http.StatusOK, "application/json", client.SessionStatusReady,
		`{"session":{"status":"ready","instances":[{"instance":{"name":"web","status":"ready","currentReplicas":1,"desiredReplicas":1}}]}}`)

	resp, err := c.StartBlocking(context.Background(), client.StartBlockingParams{
		Names:           &[]string{"web", "db"},
		SessionDuration: ptr("1m"),
		Timeout:         ptr("30s"),
	})
	if err != nil {
		t.Fatal(err)
	}

	if got.path != "/api/strategies/blocking" {
		t.Errorf("path = %q, want /api/strategies/blocking", got.path)
	}
	want := url.Values{"names": {"web", "db"}, "session_duration": {"1m"}, "timeout": {"30s"}}
	if got.query.Encode() != want.Encode() {
		t.Errorf("query = %s, want %s", got.query.Encode(), want.Encode())
	}
	if !resp.Ready() {
		t.Error("Ready() = false, want true")
	}
	session, err := resp.Session()
	if err != nil {
		t.Fatal(err)
	}
	if got := *session.Session.Status; got != "ready" {
		t.Errorf("session status = %q, want ready", got)
	}
	if got := *(*session.Session.Instances)[0].Instance.Name; got != "web" {
		t.Errorf("instance name = %q, want web", got)
	}
}

func TestStartDynamic(t *testing.T) {
	c, got := sablier(t, http.StatusOK, "text/html", client.SessionStatusNotReady, "<html>waiting</html>")

	resp, err := c.StartDynamic(context.Background(), client.StartDynamicParams{
		Group:            ptr("demo"),
		DisplayName:      ptr("My app"),
		ShowDetails:      ptr(true),
		Theme:            ptr("ghost"),
		RefreshFrequency: ptr("5s"),
	})
	if err != nil {
		t.Fatal(err)
	}

	if got.path != "/api/strategies/dynamic" {
		t.Errorf("path = %q, want /api/strategies/dynamic", got.path)
	}
	want := url.Values{
		"group":             {"demo"},
		"display_name":      {"My app"},
		"show_details":      {"true"},
		"theme":             {"ghost"},
		"refresh_frequency": {"5s"},
	}
	if got.query.Encode() != want.Encode() {
		t.Errorf("query = %s, want %s", got.query.Encode(), want.Encode())
	}
	if resp.Ready() {
		t.Error("Ready() = true, want false")
	}
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "text/html" {
		t.Errorf("status = %d, content type = %q, want 200 text/html", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	if got := string(resp.Body); got != "<html>waiting</html>" {
		t.Errorf("body = %q, want the waiting page", got)
	}
}

func TestStartPoke(t *testing.T) {
	c, got := sablier(t, http.StatusOK, "application/json", client.SessionStatusNotReady,
		`{"session":{"status":"not-ready","instances":[]}}`)

	resp, err := c.StartPoke(context.Background(), client.StartPokeParams{Group: ptr("demo")})
	if err != nil {
		t.Fatal(err)
	}

	if got.path != "/api/strategies/poke" {
		t.Errorf("path = %q, want /api/strategies/poke", got.path)
	}
	if got.query.Encode() != "group=demo" {
		t.Errorf("query = %s, want group=demo", got.query.Encode())
	}
	session, err := resp.Session()
	if err != nil {
		t.Fatal(err)
	}
	if got := *session.Session.Status; got != "not-ready" {
		t.Errorf("session status = %q, want not-ready", got)
	}
}

func TestUnsetParamsAreOmitted(t *testing.T) {
	// The server applies its defaults only to absent parameters: an empty
	// session_duration= would be bound as 0 and override the default.
	c, got := sablier(t, http.StatusOK, "application/json", client.SessionStatusReady, `{"session":{}}`)

	if _, err := c.StartBlocking(context.Background(), client.StartBlockingParams{Group: ptr("demo")}); err != nil {
		t.Fatal(err)
	}

	if got.query.Encode() != "group=demo" {
		t.Errorf("query = %s, want only group=demo", got.query.Encode())
	}
}

func TestProblem(t *testing.T) {
	c, _ := sablier(t, http.StatusNotFound, "application/problem+json", "",
		`{"type":"https://sablierapp.dev/#/errors?id=group-not-found","title":"Group not found","status":404,"detail":"The group you requested does not exist.","instance":"/api/strategies/blocking?group=nope","availableGroups":["demo"]}`)

	resp, err := c.StartBlocking(context.Background(), client.StartBlockingParams{Group: ptr("nope")})
	if err != nil {
		t.Fatalf("an HTTP error response must not be a Go error, got %v", err)
	}

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
	if resp.Ready() {
		t.Error("Ready() = true, want false for an error response")
	}
	problem := resp.Problem()
	if problem == nil {
		t.Fatalf("Problem() = nil, body: %s", resp.Body)
	}
	if got := *problem.Title; got != "Group not found" {
		t.Errorf("title = %q, want Group not found", got)
	}
	if got := problem.AdditionalProperties["availableGroups"]; got == nil {
		t.Error("availableGroups extension member was dropped")
	}
	if _, err := resp.Session(); err == nil {
		t.Error("Session() on an error response returned no error")
	}
}

func TestProblemIsNilOnSuccess(t *testing.T) {
	c, _ := sablier(t, http.StatusOK, "application/json", client.SessionStatusReady, `{"session":{}}`)

	resp, err := c.StartPoke(context.Background(), client.StartPokeParams{Group: ptr("demo")})
	if err != nil {
		t.Fatal(err)
	}

	if p := resp.Problem(); p != nil {
		t.Errorf("Problem() = %+v, want nil", p)
	}
}

func TestUserAgent(t *testing.T) {
	c, got := sablier(t, http.StatusOK, "application/json", client.SessionStatusReady, `{"session":{}}`,
		client.WithUserAgent("sablier-caddy-plugin/1.0.0"))

	if _, err := c.StartPoke(context.Background(), client.StartPokeParams{Group: ptr("demo")}); err != nil {
		t.Fatal(err)
	}

	if got.userAgent != "sablier-caddy-plugin/1.0.0" {
		t.Errorf("User-Agent = %q, want sablier-caddy-plugin/1.0.0", got.userAgent)
	}
}

func TestDefaultUserAgentHasVersion(t *testing.T) {
	c, got := sablier(t, http.StatusOK, "application/json", client.SessionStatusReady, `{"session":{}}`)

	if _, err := c.StartPoke(context.Background(), client.StartPokeParams{Group: ptr("demo")}); err != nil {
		t.Fatal(err)
	}

	if want := "sablier-go-client/" + client.Version; got.userAgent != want {
		t.Errorf("User-Agent = %q, want %q", got.userAgent, want)
	}
}

func TestBasePath(t *testing.T) {
	// Sablier can be served under a path prefix (server.base-path).
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
	}))
	t.Cleanup(srv.Close)

	c, err := client.New(srv.URL + "/sablier/")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.StartPoke(context.Background(), client.StartPokeParams{Group: ptr("demo")}); err != nil {
		t.Fatal(err)
	}

	if path != "/sablier/api/strategies/poke" {
		t.Errorf("path = %q, want /sablier/api/strategies/poke", path)
	}
}

func TestTransportError(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close()

	c, err := client.New(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.StartPoke(context.Background(), client.StartPokeParams{Group: ptr("demo")}); err == nil {
		t.Error("expected an error when Sablier is unreachable")
	}
}

func TestNewRejectsInvalidURL(t *testing.T) {
	for _, raw := range []string{"", "sablier:10000", "://bad"} {
		if _, err := client.New(raw); err == nil {
			t.Errorf("New(%q) returned no error", raw)
		}
	}
}

// roundTripFunc lets a test stand in for the HTTP transport.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestWithHTTPClient(t *testing.T) {
	var called bool
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		called = true
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: http.NoBody}, nil
	})}

	c, err := client.New("http://sablier:10000", client.WithHTTPClient(httpClient))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.StartPoke(context.Background(), client.StartPokeParams{Group: ptr("demo")}); err != nil {
		t.Fatal(err)
	}

	if !called {
		t.Error("the HTTP client passed to WithHTTPClient was not used")
	}
}

func TestMalformedBodies(t *testing.T) {
	c, _ := sablier(t, http.StatusOK, "application/problem+json", "", `not json`)

	resp, err := c.StartPoke(context.Background(), client.StartPokeParams{Group: ptr("demo")})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := resp.Session(); err == nil {
		t.Error("Session() on a malformed body returned no error")
	}
	if p := resp.Problem(); p != nil {
		t.Errorf("Problem() on a malformed body = %+v, want nil", p)
	}
}
