package proxmoxlxc

import (
	"testing"

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
