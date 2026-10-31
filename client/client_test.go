package client_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
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

	session, resp, err := c.StartBlocking(context.Background(), client.StartBlockingParams{
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
	if got := *session.Session.Status; got != "ready" {
		t.Errorf("session status = %q, want ready", got)
	}
	if got := *(*session.Session.Instances)[0].Instance.Name; got != "web" {
		t.Errorf("instance name = %q, want web", got)
	}
}

func TestStartDynamic(t *testing.T) {
	c, got := sablier(t, http.StatusOK, "text/html", client.SessionStatusNotReady, "<html>waiting</html>")

	page, resp, err := c.StartDynamic(context.Background(), client.StartDynamicParams{
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
	if got := string(page); got != "<html>waiting</html>" {
		t.Errorf("page = %q, want the waiting page", got)
	}
	if body, _ := io.ReadAll(resp.Body); string(body) != "<html>waiting</html>" {
		t.Errorf("resp.Body = %q, want the waiting page to be readable again", body)
	}
}

func TestStartPoke(t *testing.T) {
	c, got := sablier(t, http.StatusOK, "application/json", client.SessionStatusNotReady,
		`{"session":{"status":"not-ready","instances":[]}}`)

	session, _, err := c.StartPoke(context.Background(), client.StartPokeParams{Group: ptr("demo")})
	if err != nil {
		t.Fatal(err)
	}

	if got.path != "/api/strategies/poke" {
		t.Errorf("path = %q, want /api/strategies/poke", got.path)
	}
	if got.query.Encode() != "group=demo" {
		t.Errorf("query = %s, want group=demo", got.query.Encode())
	}
	if got := *session.Session.Status; got != "not-ready" {
		t.Errorf("session status = %q, want not-ready", got)
	}
}

func TestUnsetParamsAreOmitted(t *testing.T) {
	// The server applies its defaults only to absent parameters: an empty
	// session_duration= would be bound as 0 and override the default.
	c, got := sablier(t, http.StatusOK, "application/json", client.SessionStatusReady, `{"session":{}}`)

	if _, _, err := c.StartBlocking(context.Background(), client.StartBlockingParams{Group: ptr("demo")}); err != nil {
		t.Fatal(err)
	}

	if got.query.Encode() != "group=demo" {
		t.Errorf("query = %s, want only group=demo", got.query.Encode())
	}
}

func TestErrorResponse(t *testing.T) {
	const body = `{"type":"https://sablierapp.dev/#/errors?id=group-not-found","title":"Group not found","status":404,"detail":"The group you requested does not exist.","instance":"/api/strategies/blocking?group=nope","availableGroups":["demo"]}`
	c, _ := sablier(t, http.StatusNotFound, "application/problem+json", "", body)

	session, resp, err := c.StartBlocking(context.Background(), client.StartBlockingParams{Group: ptr("nope")})

	var errResp *client.ErrorResponse
	if !errors.As(err, &errResp) {
		t.Fatalf("err = %v, want a *client.ErrorResponse", err)
	}
	if session != nil {
		t.Errorf("session = %+v, want nil for an error response", session)
	}
	if resp == nil || errResp.Response != resp {
		t.Fatal("the error must carry the returned Response")
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
	if resp.Ready() {
		t.Error("Ready() = true, want false for an error response")
	}
	if got, _ := io.ReadAll(resp.Body); string(got) != body {
		t.Errorf("resp.Body = %q, want the problem as Sablier sent it", got)
	}
	problem := errResp.Problem
	if problem == nil {
		t.Fatal("Problem = nil, want the decoded problem")
	}
	if problem.Title != "Group not found" || problem.Status != http.StatusNotFound {
		t.Errorf("problem = %+v, want Group not found 404", problem)
	}
	if got := string(problem.Extensions["availableGroups"]); got != `["demo"]` {
		t.Errorf("availableGroups extension member = %q, want [\"demo\"]", got)
	}
	if _, ok := problem.Extensions["title"]; ok {
		t.Error("standard members must not be repeated in Extensions")
	}
	if want := "sablier answered 404: The group you requested does not exist."; err.Error() != want {
		t.Errorf("err = %q, want %q", err.Error(), want)
	}
}

func TestErrorResponseWithoutProblem(t *testing.T) {
	c, _ := sablier(t, http.StatusBadGateway, "text/plain", "", "bad gateway")

	_, resp, err := c.StartPoke(context.Background(), client.StartPokeParams{Group: ptr("demo")})

	var errResp *client.ErrorResponse
	if !errors.As(err, &errResp) {
		t.Fatalf("err = %v, want a *client.ErrorResponse", err)
	}
	if errResp.Problem != nil {
		t.Errorf("Problem = %+v, want nil for a body that is not a problem", errResp.Problem)
	}
	if resp.StatusCode != http.StatusBadGateway || err.Error() != "sablier answered 502" {
		t.Errorf("got %d %q, want 502 sablier answered 502", resp.StatusCode, err.Error())
	}
}

func TestUserAgent(t *testing.T) {
	c, got := sablier(t, http.StatusOK, "application/json", client.SessionStatusReady, `{"session":{}}`,
		client.WithUserAgent("sablier-caddy-plugin/1.0.0"))

	if _, _, err := c.StartPoke(context.Background(), client.StartPokeParams{Group: ptr("demo")}); err != nil {
		t.Fatal(err)
	}

	if got.userAgent != "sablier-caddy-plugin/1.0.0" {
		t.Errorf("User-Agent = %q, want sablier-caddy-plugin/1.0.0", got.userAgent)
	}
}

func TestDefaultUserAgentHasVersion(t *testing.T) {
	c, got := sablier(t, http.StatusOK, "application/json", client.SessionStatusReady, `{"session":{}}`)

	if _, _, err := c.StartPoke(context.Background(), client.StartPokeParams{Group: ptr("demo")}); err != nil {
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
		_, _ = w.Write([]byte(`{"session":{}}`))
	}))
	t.Cleanup(srv.Close)

	c, err := client.New(srv.URL + "/sablier/")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := c.StartPoke(context.Background(), client.StartPokeParams{Group: ptr("demo")}); err != nil {
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
	_, resp, err := c.StartPoke(context.Background(), client.StartPokeParams{Group: ptr("demo")})
	if err == nil {
		t.Error("expected an error when Sablier is unreachable")
	}
	if resp != nil {
		t.Errorf("resp = %+v, want nil when Sablier is unreachable", resp)
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
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"session":{}}`))}, nil
	})}

	c, err := client.New("http://sablier:10000", client.WithHTTPClient(httpClient))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := c.StartPoke(context.Background(), client.StartPokeParams{Group: ptr("demo")}); err != nil {
		t.Fatal(err)
	}

	if !called {
		t.Error("the HTTP client passed to WithHTTPClient was not used")
	}
}

func TestMalformedBodies(t *testing.T) {
	t.Run("session", func(t *testing.T) {
		c, _ := sablier(t, http.StatusOK, "application/json", "", `not json`)

		session, resp, err := c.StartPoke(context.Background(), client.StartPokeParams{Group: ptr("demo")})

		var errResp *client.ErrorResponse
		if err == nil || errors.As(err, &errResp) {
			t.Errorf("err = %v, want a decode error", err)
		}
		if session != nil || resp == nil {
			t.Errorf("got session %+v and resp %v, want no session and the response", session, resp)
		}
	})

	t.Run("problem", func(t *testing.T) {
		c, _ := sablier(t, http.StatusBadRequest, "application/problem+json", "", `not json`)

		_, _, err := c.StartPoke(context.Background(), client.StartPokeParams{Group: ptr("demo")})

		var errResp *client.ErrorResponse
		if !errors.As(err, &errResp) {
			t.Fatalf("err = %v, want a *client.ErrorResponse", err)
		}
		if errResp.Problem != nil {
			t.Errorf("Problem = %+v, want nil for a malformed problem", errResp.Problem)
		}
	})
}
