// Package yaegitest runs the client under Yaegi, the Go interpreter Traefik
// runs plugins with. Yaegi does not support all of Go, and code it cannot
// interpret makes a plugin fail to load or panic at run time. It is a
// separate module so the client itself stays standard library only.
package yaegitest

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

// script calls each endpoint and describes what it got, so the test can
// check the client behaves under Yaegi as it does compiled.
const script = `
package main

import (
	"context"
	"fmt"
	"io"

	"github.com/sablierapp/sablier/client"
)

func Blocking(url string) string {
	c, _ := client.New(url)
	timeout := "30s"
	session, resp, err := c.StartBlocking(context.Background(), client.StartBlockingParams{Names: &[]string{"web", "db"}, Timeout: &timeout})
	if err != nil {
		return err.Error()
	}
	return fmt.Sprintf("%d ready=%v session=%s", resp.StatusCode, resp.Ready(), *session.Session.Status)
}

func Dynamic(url string) string {
	c, _ := client.New(url)
	group, showDetails := "demo", true
	page, resp, err := c.StartDynamic(context.Background(), client.StartDynamicParams{Group: &group, ShowDetails: &showDetails})
	if err != nil {
		return err.Error()
	}
	body, _ := io.ReadAll(resp.Body)
	return fmt.Sprintf("%d ready=%v page=%s body=%s", resp.StatusCode, resp.Ready(), page, body)
}

func Poke(url string) string {
	c, _ := client.New(url)
	group := "nope"
	_, resp, err := c.StartPoke(context.Background(), client.StartPokeParams{Group: &group})
	errResp, ok := err.(*client.ErrorResponse)
	if !ok {
		return fmt.Sprintf("err = %v, want an *ErrorResponse", err)
	}
	return fmt.Sprintf("%d %s title=%s availableGroups=%s", resp.StatusCode, err, errResp.Problem.Title, errResp.Problem.Extensions["availableGroups"])
}

func Events(url string) string {
	c, _ := client.New(url)
	stream, _, err := c.Events(context.Background(), client.StreamInstanceEventsParams{Types: &[]string{"started"}})
	if err != nil {
		return err.Error()
	}
	defer stream.Close()
	event, err := stream.Next()
	if err != nil {
		return err.Error()
	}
	return fmt.Sprintf("%s %s", event.Type, *event.Instance.Name)
}
`

func TestClientUnderYaegi(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Query", r.URL.RawQuery)
		switch r.URL.Path {
		case "/api/strategies/blocking":
			if r.URL.RawQuery != "names=web&names=db&timeout=30s" {
				http.Error(w, "unexpected query "+r.URL.RawQuery, http.StatusTeapot)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Sablier-Session-Status", "ready")
			_, _ = io.WriteString(w, `{"session":{"status":"ready"}}`)
		case "/api/strategies/dynamic":
			if r.URL.RawQuery != "group=demo&show_details=true" {
				http.Error(w, "unexpected query "+r.URL.RawQuery, http.StatusTeapot)
				return
			}
			w.Header().Set("Content-Type", "text/html")
			w.Header().Set("X-Sablier-Session-Status", "not-ready")
			_, _ = io.WriteString(w, "<html>waiting</html>")
		case "/api/strategies/poke":
			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"title":"Group not found","status":404,"detail":"no group nope","availableGroups":["demo"]}`)
		case "/api/events":
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, "event:started\ndata:{\"name\":\"web\"}\n\n")
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	i := interp.New(interp.Options{GoPath: gopath(t)})
	if err := i.Use(stdlib.Symbols); err != nil {
		t.Fatal(err)
	}
	if _, err := i.Eval(script); err != nil {
		t.Fatalf("Yaegi cannot interpret the client: %v", err)
	}

	for name, want := range map[string]string{
		"Blocking": "200 ready=true session=ready",
		"Dynamic":  "200 ready=false page=<html>waiting</html> body=<html>waiting</html>",
		"Poke":     `404 sablier answered 404: no group nope title=Group not found availableGroups=["demo"]`,
		"Events":   "started web",
	} {
		t.Run(name, func(t *testing.T) {
			v, err := i.Eval("main." + name)
			if err != nil {
				t.Fatal(err)
			}
			if got := v.Interface().(func(string) string)(srv.URL); got != want {
				t.Errorf("got %q, want %q", got, want)
			}
		})
	}
}

// gopath lays the client sources out in a GOPATH, which is how Yaegi finds
// packages, as Traefik does with a plugin's vendor directory.
func gopath(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dst := filepath.Join(root, "src", "github.com", "sablierapp", "sablier", "client")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	sources, err := filepath.Glob("../*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, src := range sources {
		if strings.HasSuffix(src, "_test.go") {
			continue
		}
		data, err := os.ReadFile(src)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dst, filepath.Base(src)), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}
