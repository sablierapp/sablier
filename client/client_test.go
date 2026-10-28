package client_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/sablierapp/sablier/client"
)

// sablier starts a server that records the query of the last request and
// answers with the given status, content type and body.
func sablier(t *testing.T, status int, contentType string, sessionStatus string, body string) (*client.ClientWithResponses, *url.Values) {
	t.Helper()
	var query url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.Query()
		w.Header().Set("Content-Type", contentType)
		if sessionStatus != "" {
			w.Header().Set(client.SessionStatusHeader, sessionStatus)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	c, err := client.NewClientWithResponses(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	return c, &query
}

func ptr[T any](v T) *T { return &v }

func TestStartBlocking(t *testing.T) {
	c, query := sablier(t, http.StatusOK, "application/json", client.SessionStatusReady,
		`{"session":{"status":"ready","instances":[{"instance":{"name":"web","status":"ready","currentReplicas":1,"desiredReplicas":1}}]}}`)

	resp, err := c.StartBlockingWithResponse(context.Background(), &client.StartBlockingParams{
		Names:           &[]string{"web", "db"},
		SessionDuration: ptr("1m"),
		Timeout:         ptr("30s"),
	})
	if err != nil {
		t.Fatal(err)
	}

	if got := (*query)["names"]; len(got) != 2 || got[0] != "web" || got[1] != "db" {
		t.Errorf("names = %v, want each name as its own names parameter", got)
	}
	if got := query.Get("session_duration"); got != "1m" {
		t.Errorf("session_duration = %q, want 1m", got)
	}
	if got := query.Get("timeout"); got != "30s" {
		t.Errorf("timeout = %q, want 30s", got)
	}
	if !client.IsReady(resp.HTTPResponse) {
		t.Error("IsReady = false, want true")
	}
	if resp.JSON200 == nil {
		t.Fatalf("JSON200 is nil, body: %s", resp.Body)
	}
	if got := *resp.JSON200.Session.Status; got != "ready" {
		t.Errorf("session status = %q, want ready", got)
	}
	if got := *(*resp.JSON200.Session.Instances)[0].Instance.Name; got != "web" {
		t.Errorf("instance name = %q, want web", got)
	}
}

func TestStartDynamic(t *testing.T) {
	c, query := sablier(t, http.StatusOK, "text/html", client.SessionStatusNotReady, "<html>waiting</html>")

	resp, err := c.StartDynamicWithResponse(context.Background(), &client.StartDynamicParams{
		Group:            ptr("demo"),
		DisplayName:      ptr("My app"),
		ShowDetails:      ptr(true),
		Theme:            ptr("ghost"),
		RefreshFrequency: ptr("5s"),
	})
	if err != nil {
		t.Fatal(err)
	}

	want := url.Values{
		"group":             {"demo"},
		"display_name":      {"My app"},
		"show_details":      {"true"},
		"theme":             {"ghost"},
		"refresh_frequency": {"5s"},
	}
	if query.Encode() != want.Encode() {
		t.Errorf("query = %s, want %s", query.Encode(), want.Encode())
	}
	if client.IsReady(resp.HTTPResponse) {
		t.Error("IsReady = true, want false")
	}
	if got := string(resp.Body); got != "<html>waiting</html>" {
		t.Errorf("body = %q, want the waiting page", got)
	}
}

func TestStartPoke(t *testing.T) {
	c, query := sablier(t, http.StatusOK, "application/json", client.SessionStatusNotReady,
		`{"session":{"status":"not-ready","instances":[]}}`)

	resp, err := c.StartPokeWithResponse(context.Background(), &client.StartPokeParams{Group: ptr("demo")})
	if err != nil {
		t.Fatal(err)
	}

	if got := query.Get("group"); got != "demo" {
		t.Errorf("group = %q, want demo", got)
	}
	if resp.JSON200 == nil || *resp.JSON200.Session.Status != "not-ready" {
		t.Errorf("session = %+v, want not-ready", resp.JSON200)
	}
}

func TestProblemResponse(t *testing.T) {
	c, _ := sablier(t, http.StatusNotFound, "application/problem+json", "",
		`{"type":"https://sablierapp.dev/#/errors?id=group-not-found","title":"Group not found","status":404,"detail":"The group you requested does not exist.","instance":"/api/strategies/blocking?group=nope","availableGroups":["demo"]}`)

	resp, err := c.StartBlockingWithResponse(context.Background(), &client.StartBlockingParams{Group: ptr("nope")})
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode() != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode())
	}
	if client.IsReady(resp.HTTPResponse) {
		t.Error("IsReady = true, want false for an error response")
	}
	if resp.ApplicationproblemJSON404 == nil {
		t.Fatalf("problem is nil, body: %s", resp.Body)
	}
	if got := *resp.ApplicationproblemJSON404.Title; got != "Group not found" {
		t.Errorf("title = %q, want Group not found", got)
	}
	if got := resp.ApplicationproblemJSON404.AdditionalProperties["availableGroups"]; got == nil {
		t.Error("availableGroups extension member was dropped")
	}
}
