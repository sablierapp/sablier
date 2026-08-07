package proxmoxlxc_test

import (
	"testing"

	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider/proxmoxlxc"
	"gotest.tools/v3/assert"
)

func TestProxmoxLXCProvider_InstanceStop(t *testing.T) {
	t.Parallel()

	server := proxmoxlxc.MockServer(t, []string{"pve1"}, []proxmoxlxc.TestContainer{
		{VMID: 100, Name: "web", Status: "running", Tags: "sablier", Node: "pve1"},
	})
	defer server.Close()

	p, err := proxmoxlxc.New(t.Context(), proxmoxlxc.NewTestClient(server.URL), slogt.New(t))
	assert.NilError(t, err)

	err = p.InstanceStop(t.Context(), "web")
	assert.NilError(t, err)
}

func TestProxmoxLXCProvider_InstanceStop_NotFound(t *testing.T) {
	t.Parallel()

	server := proxmoxlxc.MockServer(t, []string{"pve1"}, []proxmoxlxc.TestContainer{})
	defer server.Close()

	p, err := proxmoxlxc.New(t.Context(), proxmoxlxc.NewTestClient(server.URL), slogt.New(t))
	assert.NilError(t, err)

	err = p.InstanceStop(t.Context(), "nonexistent")
	assert.ErrorContains(t, err, "not found")
}
