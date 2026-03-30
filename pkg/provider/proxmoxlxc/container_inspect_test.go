package proxmoxlxc

import (
	"testing"

	"github.com/sablierapp/sablier/pkg/sablier"
	"gotest.tools/v3/assert"
)

func TestProxmoxLXCProvider_InstanceInspect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		container testContainer
		want      sablier.InstanceInfo
	}{
		{
			name:      "running container is ready",
			container: testContainer{VMID: 100, Name: "web", Status: "running", Tags: "sablier", Node: "pve1"},
			want:      sablier.ReadyInstanceState("web", 1),
		},
		{
			name:      "stopped container is not ready",
			container: testContainer{VMID: 101, Name: "db", Status: "stopped", Tags: "sablier", Node: "pve1"},
			want:      sablier.NotReadyInstanceState("db", 0, 1),
		},
		{
			name:      "unknown status is unrecoverable",
			container: testContainer{VMID: 102, Name: "backup", Status: "unknown", Tags: "sablier", Node: "pve1"},
			want:      sablier.UnrecoverableInstanceState("backup", "container status \"unknown\" not handled", 1),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := mockServer(t, []string{"pve1"}, []testContainer{tt.container})
			defer server.Close()

			p := newTestProvider(t, server.URL)
			got, err := p.InstanceInspect(t.Context(), tt.container.Name)
			assert.NilError(t, err)
			assert.DeepEqual(t, got, tt.want)
		})
	}
}

func TestProxmoxLXCProvider_InstanceInspect_ByVMID(t *testing.T) {
	t.Parallel()

	server := mockServer(t, []string{"pve1"}, []testContainer{
		{VMID: 100, Name: "web", Status: "running", Tags: "sablier", Node: "pve1"},
	})
	defer server.Close()

	p := newTestProvider(t, server.URL)

	// Resolve by VMID string
	got, err := p.InstanceInspect(t.Context(), "100")
	assert.NilError(t, err)
	assert.DeepEqual(t, got, sablier.ReadyInstanceState("web", 1))
}

func TestProxmoxLXCProvider_InstanceInspect_ByNodeVMID(t *testing.T) {
	t.Parallel()

	server := mockServer(t, []string{"pve1"}, []testContainer{
		{VMID: 100, Name: "web", Status: "running", Tags: "sablier", Node: "pve1"},
	})
	defer server.Close()

	p := newTestProvider(t, server.URL)

	// Resolve by "node/vmid" format
	got, err := p.InstanceInspect(t.Context(), "pve1/100")
	assert.NilError(t, err)
	// node/vmid resolves to the hostname via scanContainers.
	assert.DeepEqual(t, got, sablier.ReadyInstanceState("web", 1))
}

func TestProxmoxLXCProvider_InstanceInspect_NotFound(t *testing.T) {
	t.Parallel()

	server := mockServer(t, []string{"pve1"}, []testContainer{})
	defer server.Close()

	p := newTestProvider(t, server.URL)
	_, err := p.InstanceInspect(t.Context(), "nonexistent")
	assert.ErrorContains(t, err, "not found")
}
