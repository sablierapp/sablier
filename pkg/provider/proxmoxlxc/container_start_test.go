package proxmoxlxc

import (
	"testing"
	"time"

	"gotest.tools/v3/assert"
)

func TestProxmoxLXCProvider_InstanceStart(t *testing.T) {
	t.Parallel()

	server := mockServer(t, []string{"pve1"}, []testContainer{
		{VMID: 100, Name: "web", Status: "stopped", Tags: "sablier", Node: "pve1"},
	})
	defer server.Close()

	p := newTestProvider(t, server.URL)
	err := p.InstanceStart(t.Context(), "web")
	assert.NilError(t, err)

	// Verify the task is stored in pendingTasks.
	p.mu.RLock()
	_, hasPending := p.pendingTasks["web"]
	p.mu.RUnlock()
	assert.Assert(t, hasPending, "expected pending task to be stored")
}

func TestProxmoxLXCProvider_InstanceStart_ByVMID(t *testing.T) {
	t.Parallel()

	server := mockServer(t, []string{"pve1"}, []testContainer{
		{VMID: 100, Name: "web", Status: "stopped", Tags: "sablier", Node: "pve1"},
	})
	defer server.Close()

	p := newTestProvider(t, server.URL)
	err := p.InstanceStart(t.Context(), "100")
	assert.NilError(t, err)
}

func TestProxmoxLXCProvider_InstanceStart_NotFound(t *testing.T) {
	t.Parallel()

	server := mockServer(t, []string{"pve1"}, []testContainer{})
	defer server.Close()

	p := newTestProvider(t, server.URL)
	err := p.InstanceStart(t.Context(), "nonexistent")
	assert.ErrorContains(t, err, "not found")
}

func TestProxmoxLXCProvider_InstanceStart_AlreadyRunning(t *testing.T) {
	t.Parallel()

	server := mockServer(t, []string{"pve1"}, []testContainer{
		{VMID: 100, Name: "web", Status: "running", Tags: "sablier", Node: "pve1"},
	})
	defer server.Close()

	p := newTestProvider(t, server.URL)
	err := p.InstanceStart(t.Context(), "web")
	assert.NilError(t, err)

	// No task should be stored for an already running container.
	p.mu.RLock()
	_, hasPending := p.pendingTasks["web"]
	p.mu.RUnlock()
	assert.Assert(t, !hasPending, "expected no pending task for already running container")
}

func TestProxmoxLXCProvider_InstanceStart_TaskFailure(t *testing.T) {
	t.Parallel()

	server := mockServer(t, []string{"pve1"}, []testContainer{
		{VMID: 100, Name: "broken", Status: "stopped", Tags: "sablier", Node: "pve1",
			StartTaskState: taskFailed, StartTaskExitStatus: "startup for container '100' failed"},
	})
	defer server.Close()

	p := newTestProvider(t, server.URL)

	// InstanceStart should return nil (task is stored, not awaited).
	err := p.InstanceStart(t.Context(), "broken")
	assert.NilError(t, err)

	// InstanceInspect should detect the failed task via Ping() and report unrecoverable state.
	got, err := p.InstanceInspect(t.Context(), "broken")
	assert.NilError(t, err)
	assert.Equal(t, string(got.Status), "unrecoverable")
	assert.Assert(t, got.Message != "", "expected a non-empty error message")

	// Pending task should be cleaned up after failure detection.
	p.mu.RLock()
	_, hasPending := p.pendingTasks["broken"]
	p.mu.RUnlock()
	assert.Assert(t, !hasPending, "expected pending task to be removed after failure")
}

func TestProxmoxLXCProvider_InstanceStart_TaskFailureTTLExpiry(t *testing.T) {
	t.Parallel()

	// Set the task end time far enough in the past that the TTL (30s) has expired.
	server := proxmoxlxc.MockServer(t, []string{"pve1"}, []proxmoxlxc.TestContainer{
		{VMID: 100, Name: "broken", Status: "stopped", Tags: "sablier", Node: "pve1",
			StartTaskState:      proxmoxlxc.TaskFailed,
			StartTaskExitStatus: "startup for container '100' failed",
			StartTaskEndTime:    time.Now().Add(-time.Minute)},
	})
	defer server.Close()

	p, err := proxmoxlxc.New(t.Context(), proxmoxlxc.NewTestClient(server.URL), slogt.New(t))
	assert.NilError(t, err)

	err = p.InstanceStart(t.Context(), "broken")
	assert.NilError(t, err)

	// First InstanceInspect triggers Ping which discovers the failure.
	// Since EndTime is >30s ago, the failed entry should be cleared and
	// the provider should fall through to the normal container status check.
	got, err := p.InstanceInspect(t.Context(), "broken")
	assert.NilError(t, err)
	assert.Equal(t, string(got.Status), "not-ready", "expected not-ready after TTL expiry, got %s", got.Status)
}

func TestProxmoxLXCProvider_InstanceStart_TaskInProgress(t *testing.T) {
	t.Parallel()

	server := mockServer(t, []string{"pve1"}, []testContainer{
		{VMID: 100, Name: "slow", Status: "stopped", Tags: "sablier", Node: "pve1",
			StartTaskState: taskRunning},
	})
	defer server.Close()

	p := newTestProvider(t, server.URL)

	err := p.InstanceStart(t.Context(), "slow")
	assert.NilError(t, err)

	// InstanceInspect should report not-ready while the task is still running.
	got, err := p.InstanceInspect(t.Context(), "slow")
	assert.NilError(t, err)
	assert.Equal(t, string(got.Status), "not-ready")

	// Pending task should still be stored.
	p.mu.RLock()
	_, hasPending := p.pendingTasks["slow"]
	p.mu.RUnlock()
	assert.Assert(t, hasPending, "expected pending task to remain while task is running")
}
