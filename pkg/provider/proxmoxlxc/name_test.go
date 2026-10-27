package proxmoxlxc_test

import (
	"testing"

	"github.com/neilotoole/slogt"
	"gotest.tools/v3/assert"

	"github.com/sablierapp/sablier/pkg/provider/proxmoxlxc"
	"github.com/sablierapp/sablier/pkg/sablier"
)

var _ sablier.InstanceNameResolver = (*proxmoxlxc.Provider)(nil)

// A request may name a container by its ID, but inspect and the session store
// use its hostname. See #1178.
func TestProxmoxLXCProvider_InstanceName(t *testing.T) {
	t.Parallel()

	server := proxmoxlxc.MockServer(t, []string{"pve1"}, []proxmoxlxc.TestContainer{
		{VMID: 100, Name: "web", Status: "running", Tags: "sablier", Node: "pve1"},
	})
	p, err := proxmoxlxc.New(t.Context(), proxmoxlxc.NewTestClient(server.URL), slogt.New(t))
	assert.NilError(t, err)

	tests := map[string]string{
		"100":      "web",
		"pve1/100": "web",
		"web":      "web",
		"unknown":  "unknown",
	}
	for name, want := range tests {
		assert.Equal(t, p.InstanceName(t.Context(), name), want, name)
	}
}
