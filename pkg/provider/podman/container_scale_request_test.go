package podman_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/moby/moby/client"
	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider/podman"
	"gotest.tools/v3/assert"
)

// Podman's Docker-compatible update ignores NanoCPUs and only reads CpuQuota
// and CpuPeriod, so scale mode must send the CPU limit that way. This test
// checks the request itself, independently of the container runtime. See #1189.
func TestPodmanScaleMode_SendsCPUAsQuota(t *testing.T) {
	var mu sync.Mutex
	var update map[string]any

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/version"):
			_ = json.NewEncoder(w).Encode(map[string]any{"Version": "5.8.2", "ApiVersion": "1.41", "MinAPIVersion": "1.24"})
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/containers/web/json"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"Id":     "web",
				"Name":   "/web",
				"State":  map[string]any{"Status": "running", "Running": true},
				"Config": map[string]any{"Labels": map[string]string{"sablier.idle.replicas": "1", "sablier.idle.cpu": "0.5"}},
			})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/containers/web/update"):
			mu.Lock()
			defer mu.Unlock()
			assert.NilError(t, json.NewDecoder(r.Body).Decode(&update))
			_ = json.NewEncoder(w).Encode(map[string]any{"Warnings": []string{}})
		default:
			http.NotFound(w, r)
		}
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	cli, err := client.New(client.WithHost("tcp://"+strings.TrimPrefix(server.URL, "http://")), client.WithAPIVersion("1.41"))
	assert.NilError(t, err)
	p, err := podman.New(t.Context(), cli, slogt.New(t))
	assert.NilError(t, err)

	assert.NilError(t, p.InstanceStop(t.Context(), "web"))

	mu.Lock()
	defer mu.Unlock()
	assert.Assert(t, update != nil, "no update request was sent")
	assert.Equal(t, update["CpuPeriod"], float64(100000), "update: %v", update)
	assert.Equal(t, update["CpuQuota"], float64(50000), "update: %v", update)
	nano, _ := update["NanoCpus"].(float64)
	assert.Equal(t, nano, float64(0), "update: %v", update)
}
