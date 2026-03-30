package proxmoxlxc

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	proxmox "github.com/luthermonson/go-proxmox"
)

// proxmoxResponse wraps data in the Proxmox API JSON envelope.
func proxmoxResponse(data interface{}) []byte {
	b, _ := json.Marshal(map[string]interface{}{"data": data})
	return b
}

// writeJSON writes a Proxmox API JSON response to w, failing the test on error.
func writeJSON(t *testing.T, w http.ResponseWriter, data interface{}) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(proxmoxResponse(data)); err != nil {
		t.Errorf("failed to write mock response: %v", err)
	}
}

// testContainer represents a container in the mock API.
type testContainer struct {
	VMID               int    `json:"vmid"`
	Name               string `json:"name"`
	Status             string `json:"status"`
	Tags               string `json:"tags"`
	Node               string `json:"-"` // not sent in API response, used for routing
	StartTaskExitStatus string `json:"-"` // non-empty overrides the task exit status (default "OK")
}

// mockServer sets up a mock Proxmox API server with the given nodes and containers.
func mockServer(t *testing.T, nodes []string, containers []testContainer) *httptest.Server {
	t.Helper()

	// Build per-node container lists
	nodeContainers := make(map[string][]testContainer)
	for _, c := range containers {
		nodeContainers[c.Node] = append(nodeContainers[c.Node], c)
	}

	mux := http.NewServeMux()

	// GET /api2/json/version
	mux.HandleFunc("GET /api2/json/version", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]string{
			"version": "8.2-1",
			"release": "8.2",
			"repoid":  "test",
		})
	})

	// GET /api2/json/nodes
	mux.HandleFunc("GET /api2/json/nodes", func(w http.ResponseWriter, r *http.Request) {
		var ns []map[string]interface{}
		for _, n := range nodes {
			ns = append(ns, map[string]interface{}{
				"node":   n,
				"status": "online",
				"type":   "node",
			})
		}
		writeJSON(t, w, ns)
	})

	// Per-node container list and operations
	for _, nodeName := range nodes {
		node := nodeName // capture

		// GET /api2/json/nodes/{node}/status (called by client.Node())
		mux.HandleFunc(fmt.Sprintf("GET /api2/json/nodes/%s/status", node), func(w http.ResponseWriter, r *http.Request) {
			writeJSON(t, w, map[string]interface{}{"node": node, "status": "online"})
		})

		// GET /api2/json/nodes/{node}/lxc
		mux.HandleFunc(fmt.Sprintf("GET /api2/json/nodes/%s/lxc", node), func(w http.ResponseWriter, r *http.Request) {
			writeJSON(t, w, nodeContainers[node])
		})

		// GET /api2/json/nodes/{node}/lxc/{vmid}/status/current and /config
		for _, ct := range nodeContainers[node] {
			c := ct // capture
			mux.HandleFunc(fmt.Sprintf("GET /api2/json/nodes/%s/lxc/%d/status/current", node, c.VMID), func(w http.ResponseWriter, r *http.Request) {
				writeJSON(t, w, map[string]interface{}{
					"vmid":   c.VMID,
					"name":   c.Name,
					"status": c.Status,
					"tags":   c.Tags,
					"node":   node,
				})
			})

			// GET /api2/json/nodes/{node}/lxc/{vmid}/config (called by node.Container())
			mux.HandleFunc(fmt.Sprintf("GET /api2/json/nodes/%s/lxc/%d/config", node, c.VMID), func(w http.ResponseWriter, r *http.Request) {
				writeJSON(t, w, map[string]interface{}{
					"hostname": c.Name,
					"tags":     c.Tags,
				})
			})

			// GET /api2/json/nodes/{node}/lxc/{vmid}/interfaces
			mux.HandleFunc(fmt.Sprintf("GET /api2/json/nodes/%s/lxc/%d/interfaces", node, c.VMID), func(w http.ResponseWriter, r *http.Request) {
				if c.Status == "running" {
					writeJSON(t, w, []map[string]string{
						{"name": "lo", "hwaddr": "00:00:00:00:00:00", "inet": "127.0.0.1/8", "inet6": "::1/128"},
						{"name": "eth0", "hwaddr": "bc:24:11:89:67:07", "inet": "192.168.1.100/24", "inet6": "fe80::1/64"},
					})
				} else {
					// Stopped containers have no interfaces
					writeJSON(t, w, []map[string]string{})
				}
			})

			// POST /api2/json/nodes/{node}/lxc/{vmid}/status/start
			mux.HandleFunc(fmt.Sprintf("POST /api2/json/nodes/%s/lxc/%d/status/start", node, c.VMID), func(w http.ResponseWriter, r *http.Request) {
				upid := fmt.Sprintf("UPID:%s:%08X:%08X:%08X:vzstart:%d:root@pam:", node, 1, 1, 1, c.VMID)
				writeJSON(t, w, upid)
			})

			// POST /api2/json/nodes/{node}/lxc/{vmid}/status/stop
			mux.HandleFunc(fmt.Sprintf("POST /api2/json/nodes/%s/lxc/%d/status/stop", node, c.VMID), func(w http.ResponseWriter, r *http.Request) {
				upid := fmt.Sprintf("UPID:%s:%08X:%08X:%08X:vzstop:%d:root@pam:", node, 1, 1, 1, c.VMID)
				writeJSON(t, w, upid)
			})
		}

		// Build a VMID → exit status map for this node.
		taskExitStatus := make(map[string]string) // vmid string → exit status
		for _, c := range nodeContainers[node] {
			if c.StartTaskExitStatus != "" {
				taskExitStatus[fmt.Sprintf("%d", c.VMID)] = c.StartTaskExitStatus
			}
		}

		// GET /api2/json/nodes/{node}/tasks/{upid}/status - task status
		// The Task.UnmarshalJSON uses copier.Copy which zeroes fields not in the response,
		// so we must include upid/node/type/id/user to preserve them (matching real Proxmox API).
		tasksPrefix := fmt.Sprintf("/api2/json/nodes/%s/tasks/", node)
		mux.HandleFunc(fmt.Sprintf("GET %s", tasksPrefix), func(w http.ResponseWriter, r *http.Request) {
			// Extract UPID from the URL path: /api2/json/nodes/{node}/tasks/{upid}/status
			rest := strings.TrimPrefix(r.URL.Path, tasksPrefix)
			upid := strings.TrimSuffix(rest, "/status")

			// Determine exit status from UPID (format: UPID:node:pid:pstart:time:type:vmid:user:)
			exitStatus := "OK"
			parts := strings.Split(upid, ":")
			if len(parts) >= 7 {
				vmid := parts[6]
				if es, ok := taskExitStatus[vmid]; ok {
					exitStatus = es
				}
			}

			writeJSON(t, w, map[string]interface{}{
				"status":     "stopped",
				"exitstatus": exitStatus,
				"upid":       upid,
				"node":       node,
				"type":       "lxcstart",
				"id":         "100",
				"user":       "root@pam",
			})
		})
	}

	return httptest.NewServer(mux)
}

// newTestProvider creates a Provider connected to the mock server.
func newTestProvider(t *testing.T, serverURL string) *Provider {
	t.Helper()
	client := proxmox.NewClient(
		serverURL+"/api2/json",
		proxmox.WithAPIToken("test@pam!test", "test-secret"),
	)
	return &Provider{
		client:          client,
		l:               slog.Default(),
		desiredReplicas: 1,
		pollInterval:    50 * time.Millisecond,
		cache:           make(map[string]containerRef),
		failedStarts:    make(map[string]string),
	}
}
