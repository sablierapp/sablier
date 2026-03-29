package proxmoxlxc

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestProxmoxLXCProvider_InstanceStop(t *testing.T) {
	t.Parallel()

	server := mockServer(t, []string{"pve1"}, []testContainer{
		{VMID: 100, Name: "web", Status: "running", Tags: "sablier", Node: "pve1"},
	})
	defer server.Close()

	p := newTestProvider(t, server.URL)
	err := p.InstanceStop(t.Context(), "web")
	assert.NilError(t, err)
}

func TestProxmoxLXCProvider_InstanceStop_NotFound(t *testing.T) {
	t.Parallel()

	server := mockServer(t, []string{"pve1"}, []testContainer{})
	defer server.Close()

	p := newTestProvider(t, server.URL)
	err := p.InstanceStop(t.Context(), "nonexistent")
	assert.ErrorContains(t, err, "not found")
}
