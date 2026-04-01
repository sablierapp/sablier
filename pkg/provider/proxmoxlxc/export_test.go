package proxmoxlxc

import (
	"context"
	"log/slog"
	"net/http/httptest"
	"testing"
	"time"

	proxmox "github.com/luthermonson/go-proxmox"
)

// Exported aliases for external test packages.

// TaskState controls how the mock API reports a task's status.
type TaskState = taskState

const (
	TaskCompleted TaskState = taskCompleted
	TaskRunning   TaskState = taskRunning
	TaskFailed    TaskState = taskFailed
)

// TestContainer represents a container in the mock API.
type TestContainer = testContainer

// MockServer sets up a mock Proxmox API server with the given nodes and containers.
func MockServer(t *testing.T, nodes []string, containers []TestContainer) *httptest.Server {
	return mockServer(t, nodes, containers)
}

// NewTestClient creates a Proxmox client configured for the mock server.
func NewTestClient(serverURL string) *proxmox.Client {
	return proxmox.NewClient(
		serverURL+"/api2/json",
		proxmox.WithAPIToken("test@pam!test", "test-secret"),
	)
}

// NewForTest creates a Provider with a custom poll interval for testing.
func NewForTest(ctx context.Context, client *proxmox.Client, logger *slog.Logger, pollInterval time.Duration) (*Provider, error) {
	p, err := New(ctx, client, logger)
	if err != nil {
		return nil, err
	}
	p.pollInterval = pollInterval
	return p, nil
}
