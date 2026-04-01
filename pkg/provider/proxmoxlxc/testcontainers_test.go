package proxmoxlxc

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	proxmox "github.com/luthermonson/go-proxmox"
	"github.com/neilotoole/slogt"
)

// proxmoxTestEnv holds the state for an integration test against a real Proxmox VE.
type proxmoxTestEnv struct {
	provider *Provider
	node     string
	vmid     int    // VMID of the cloned test container
	name     string // hostname of the cloned test container
}

// setupProxmox creates a real Proxmox LXC container by cloning a template,
// tags it with "sablier" and "sablier-group-test", and returns a test environment.
// The cloned container is automatically destroyed via t.Cleanup.
//
// Required environment variables (test is skipped if any are missing):
//
//	PROXMOX_URL              - Proxmox API URL (e.g. https://proxmox.local:8006/api2/json)
//	PROXMOX_TOKEN_ID         - API token ID (e.g. root@pam!sablier)
//	PROXMOX_TOKEN_SECRET     - API token secret
//	PROXMOX_TEST_NODE        - Node name to run tests on
//	PROXMOX_TEST_TEMPLATE_VMID - VMID of the LXC template to clone
//
// Optional:
//
//	PROXMOX_TLS_INSECURE     - Set to "true" to skip TLS verification
func setupProxmox(t *testing.T) *proxmoxTestEnv {
	t.Helper()

	apiURL := os.Getenv("PROXMOX_URL")
	tokenID := os.Getenv("PROXMOX_TOKEN_ID")
	tokenSecret := os.Getenv("PROXMOX_TOKEN_SECRET")
	nodeName := os.Getenv("PROXMOX_TEST_NODE")
	templateVMIDStr := os.Getenv("PROXMOX_TEST_TEMPLATE_VMID")

	if apiURL == "" || tokenID == "" || tokenSecret == "" || nodeName == "" || templateVMIDStr == "" {
		t.Skip("skipping integration test: PROXMOX_URL, PROXMOX_TOKEN_ID, PROXMOX_TOKEN_SECRET, PROXMOX_TEST_NODE, PROXMOX_TEST_TEMPLATE_VMID must be set")
	}

	templateVMID, err := strconv.Atoi(templateVMIDStr)
	if err != nil {
		t.Fatalf("PROXMOX_TEST_TEMPLATE_VMID must be an integer: %v", err)
	}

	ctx := t.Context()

	// Build client options
	opts := []proxmox.Option{
		proxmox.WithAPIToken(tokenID, tokenSecret),
	}
	if os.Getenv("PROXMOX_TLS_INSECURE") == "true" {
		opts = append(opts, proxmox.WithHTTPClient(&http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true, //nolint:gosec // intentional for test environments
				},
			},
		}))
	}

	client := proxmox.NewClient(apiURL, opts...)

	// Verify connectivity
	version, err := client.Version(ctx)
	if err != nil {
		t.Fatalf("cannot connect to Proxmox VE at %s: %v", apiURL, err)
	}
	t.Logf("connected to Proxmox VE %s (release %s)", version.Version, version.Release)

	// Get the node and template container
	node, err := client.Node(ctx, nodeName)
	if err != nil {
		t.Fatalf("cannot get node %q: %v", nodeName, err)
	}

	template, err := node.Container(ctx, templateVMID)
	if err != nil {
		t.Fatalf("cannot get template container %d: %v", templateVMID, err)
	}

	// Clone the template
	hostname := fmt.Sprintf("sablier-test-%d", time.Now().UnixMilli()%100000)
	newID, task, err := template.Clone(ctx, &proxmox.ContainerCloneOptions{
		Hostname: hostname,
	})
	if err != nil {
		t.Fatalf("cannot clone template %d: %v", templateVMID, err)
	}

	t.Logf("cloning template %d → VMID %d (hostname %s)", templateVMID, newID, hostname)
	if err := task.Wait(ctx, 2*time.Second, 120*time.Second); err != nil {
		t.Fatalf("clone task failed: %v", err)
	}

	// Get the cloned container
	ct, err := node.Container(ctx, newID)
	if err != nil {
		t.Fatalf("cannot get cloned container %d: %v", newID, err)
	}

	// Add sablier tags
	for _, tag := range []string{"sablier", "sablier-group-test"} {
		tagTask, err := ct.AddTag(ctx, tag)
		if err != nil {
			t.Fatalf("cannot add tag %q to container %d: %v", tag, newID, err)
		}
		if tagTask != nil {
			if err := tagTask.Wait(ctx, 1*time.Second, 30*time.Second); err != nil {
				t.Fatalf("add tag %q task failed: %v", tag, err)
			}
		}
	}

	t.Logf("test container ready: VMID %d, hostname %s, tags: sablier;sablier-group-test", newID, hostname)

	// Register cleanup: stop (if running) and destroy the cloned container.
	// Use a background context because t.Context() is canceled when the test finishes.
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		// Re-fetch the container to get current status
		ct, err := node.Container(cleanupCtx, newID)
		if err != nil {
			t.Logf("cleanup: cannot get container %d (may already be deleted): %v", newID, err)
			return
		}

		if ct.Status == "running" {
			stopTask, err := ct.Stop(cleanupCtx)
			if err != nil {
				t.Logf("cleanup: cannot stop container %d: %v", newID, err)
			} else if err := stopTask.Wait(cleanupCtx, 2*time.Second, 60*time.Second); err != nil {
				t.Logf("cleanup: stop task failed for container %d: %v", newID, err)
			}
		}

		delTask, err := ct.Delete(cleanupCtx)
		if err != nil {
			t.Logf("cleanup: cannot delete container %d: %v", newID, err)
			return
		}
		if err := delTask.Wait(cleanupCtx, 2*time.Second, 60*time.Second); err != nil {
			t.Logf("cleanup: delete task failed for container %d: %v", newID, err)
		}
		t.Logf("cleanup: container %d deleted", newID)
	})

	// Create the provider
	logger := slogt.New(t).With(slog.String("provider", "proxmox_lxc"))
	provider := &Provider{
		client:          client,
		l:               logger,
		desiredReplicas: 1,
		pollInterval:    2 * time.Second,
		cache:           make(map[string]containerRef),
		pendingTasks:    make(map[string]*proxmox.Task),
	}

	return &proxmoxTestEnv{
		provider: provider,
		node:     nodeName,
		vmid:     newID,
		name:     hostname,
	}
}
