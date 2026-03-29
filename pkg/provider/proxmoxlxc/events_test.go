package proxmoxlxc

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	proxmox "github.com/luthermonson/go-proxmox"
	"gotest.tools/v3/assert"
)

func TestProxmoxLXCProvider_NotifyInstanceStopped(t *testing.T) {
	t.Parallel()

	// Track the current status of the container, allowing us to change it mid-test
	var mu sync.Mutex
	containerStatus := "running"

	mux := http.NewServeMux()

	mux.HandleFunc("GET /api2/json/version", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]string{"version": "8.2-1", "release": "8.2"})
	})

	mux.HandleFunc("GET /api2/json/nodes", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, []map[string]interface{}{{"node": "pve1", "status": "online"}})
	})

	mux.HandleFunc("GET /api2/json/nodes/pve1/status", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]interface{}{"node": "pve1", "status": "online"})
	})

	mux.HandleFunc("GET /api2/json/nodes/pve1/lxc", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		status := containerStatus
		mu.Unlock()
		writeJSON(t, w, []map[string]interface{}{
			{"vmid": 100, "name": "web", "status": status, "tags": "sablier"},
		})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client := proxmox.NewClient(
		server.URL+"/api2/json",
		proxmox.WithAPIToken("test@pam!test", "test-secret"),
	)
	p := &Provider{
		client:          client,
		l:               slog.Default(),
		desiredReplicas: 1,
		pollInterval:    50 * time.Millisecond,
		cache:           make(map[string]containerRef),
	}

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	stoppedCh := make(chan string, 1)
	go p.NotifyInstanceStopped(ctx, stoppedCh)

	// Wait for initial scan
	time.Sleep(100 * time.Millisecond)

	// Simulate container stop
	mu.Lock()
	containerStatus = "stopped"
	mu.Unlock()

	// Wait for the notification
	select {
	case name := <-stoppedCh:
		assert.Equal(t, name, "web")
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for stopped notification")
	}
}

func TestProxmoxLXCProvider_NotifyInstanceStopped_ContextCancel(t *testing.T) {
	t.Parallel()

	server := mockServer(t, []string{"pve1"}, []testContainer{
		{VMID: 100, Name: "web", Status: "running", Tags: "sablier", Node: "pve1"},
	})
	defer server.Close()

	p := newTestProvider(t, server.URL)

	ctx, cancel := context.WithCancel(t.Context())
	stoppedCh := make(chan string, 1)
	go p.NotifyInstanceStopped(ctx, stoppedCh)

	// Cancel context and verify channel is closed
	cancel()
	select {
	case _, ok := <-stoppedCh:
		if ok {
			// Got a value, that's unexpected but not fatal — just wait for close
			_, ok = <-stoppedCh
		}
		assert.Assert(t, !ok, "channel should be closed after context cancel")
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for channel close")
	}
}
