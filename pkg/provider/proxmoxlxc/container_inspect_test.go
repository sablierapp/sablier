package proxmoxlxc_test

import (
	"testing"

	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider/proxmoxlxc"
	"github.com/sablierapp/sablier/pkg/sablier"
	"gotest.tools/v3/assert"
)

func TestProxmoxLXCProvider_InstanceInspect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		container proxmoxlxc.TestContainer
		want      sablier.InstanceInfo
	}{
		{
			name:      "running container is ready",
			container: proxmoxlxc.TestContainer{VMID: 100, Name: "web", Status: "running", Tags: "sablier", Node: "pve1"},
			want: sablier.InstanceInfo{
				Name:            "web",
				CurrentReplicas: 1,
				DesiredReplicas: 1,
				Status:          sablier.InstanceStatusReady,
				Enabled:         "true",
				Groups:          []string{"default"},
			},
		},
		{
			name:      "stopped container is not ready",
			container: proxmoxlxc.TestContainer{VMID: 101, Name: "db", Status: "stopped", Tags: "sablier", Node: "pve1"},
			want: sablier.InstanceInfo{
				Name:            "db",
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          sablier.InstanceStatusStopped,
				Enabled:         "true",
				Groups:          []string{"default"},
			},
		},
		{
			name:      "unknown status is unrecoverable",
			container: proxmoxlxc.TestContainer{VMID: 102, Name: "backup", Status: "unknown", Tags: "sablier", Node: "pve1"},
			want: sablier.InstanceInfo{
				Name:            "backup",
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          sablier.InstanceStatusError,
				Enabled:         "true",
				Groups:          []string{"default"},
				Message:         "container status \"unknown\" not handled",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := proxmoxlxc.MockServer(t, []string{"pve1"}, []proxmoxlxc.TestContainer{tt.container})

			p, err := proxmoxlxc.New(t.Context(), proxmoxlxc.NewTestClient(server.URL), slogt.New(t))
			assert.NilError(t, err)

			got, err := p.InstanceInspect(t.Context(), tt.container.Name)
			assert.NilError(t, err)
			assert.DeepEqual(t, got, tt.want)
		})
	}
}

func TestProxmoxLXCProvider_InstanceInspect_ByVMID(t *testing.T) {
	t.Parallel()

	server := proxmoxlxc.MockServer(t, []string{"pve1"}, []proxmoxlxc.TestContainer{
		{VMID: 100, Name: "web", Status: "running", Tags: "sablier", Node: "pve1"},
	})

	p, err := proxmoxlxc.New(t.Context(), proxmoxlxc.NewTestClient(server.URL), slogt.New(t))
	assert.NilError(t, err)

	got, err := p.InstanceInspect(t.Context(), "100")
	assert.NilError(t, err)
	assert.DeepEqual(t, got, sablier.InstanceInfo{
		Name:            "web",
		CurrentReplicas: 1,
		DesiredReplicas: 1,
		Status:          sablier.InstanceStatusReady,
		Enabled:         "true",
		Groups:          []string{"default"},
	})
}

func TestProxmoxLXCProvider_InstanceInspect_ByNodeVMID(t *testing.T) {
	t.Parallel()

	server := proxmoxlxc.MockServer(t, []string{"pve1"}, []proxmoxlxc.TestContainer{
		{VMID: 100, Name: "web", Status: "running", Tags: "sablier", Node: "pve1"},
	})

	p, err := proxmoxlxc.New(t.Context(), proxmoxlxc.NewTestClient(server.URL), slogt.New(t))
	assert.NilError(t, err)

	got, err := p.InstanceInspect(t.Context(), "pve1/100")
	assert.NilError(t, err)
	// node/vmid resolves to the hostname via scanContainers.
	assert.DeepEqual(t, got, sablier.InstanceInfo{
		Name:            "web",
		CurrentReplicas: 1,
		DesiredReplicas: 1,
		Status:          sablier.InstanceStatusReady,
		Enabled:         "true",
		Groups:          []string{"default"},
	})
}

func TestProxmoxLXCProvider_InstanceInspect_NotFound(t *testing.T) {
	t.Parallel()

	server := proxmoxlxc.MockServer(t, []string{"pve1"}, []proxmoxlxc.TestContainer{})

	p, err := proxmoxlxc.New(t.Context(), proxmoxlxc.NewTestClient(server.URL), slogt.New(t))
	assert.NilError(t, err)

	_, err = p.InstanceInspect(t.Context(), "nonexistent")
	assert.ErrorContains(t, err, "not found")
}

// Sablier's core checks IsEnabled on inspected instances (reject-unlabeled-requests,
// verify-enabled-on-expiration), so inspect must report the sablier tag. See #1177.
func TestProxmoxLXCProvider_InstanceInspect_EnabledFromTags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		container   proxmoxlxc.TestContainer
		wantEnabled bool
		wantGroups  []string
	}{
		{
			name:        "tagged container is enabled with its groups",
			container:   proxmoxlxc.TestContainer{VMID: 100, Name: "web", Status: "stopped", Tags: "sablier;sablier-group-web", Node: "pve1"},
			wantEnabled: true,
			wantGroups:  []string{"web"},
		},
		{
			name:        "untagged container is not enabled",
			container:   proxmoxlxc.TestContainer{VMID: 101, Name: "other", Status: "stopped", Tags: "", Node: "pve1"},
			wantEnabled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := proxmoxlxc.MockServer(t, []string{"pve1"}, []proxmoxlxc.TestContainer{tt.container})
			p, err := proxmoxlxc.New(t.Context(), proxmoxlxc.NewTestClient(server.URL), slogt.New(t))
			assert.NilError(t, err)

			got, err := p.InstanceInspect(t.Context(), tt.container.Name)
			assert.NilError(t, err)
			assert.Equal(t, got.IsEnabled(), tt.wantEnabled)
			if tt.wantEnabled {
				assert.DeepEqual(t, got.Groups, tt.wantGroups)
			}
		})
	}
}

// A second request while the start task runs inspects through the pending-task
// path, which must report the tag too, or reject-unlabeled-requests rejects it.
func TestProxmoxLXCProvider_InstanceInspect_EnabledWhileStartPending(t *testing.T) {
	t.Parallel()

	server := proxmoxlxc.MockServer(t, []string{"pve1"}, []proxmoxlxc.TestContainer{
		{VMID: 100, Name: "web", Status: "stopped", Tags: "sablier", Node: "pve1", StartTaskState: proxmoxlxc.TaskRunning},
	})
	p, err := proxmoxlxc.New(t.Context(), proxmoxlxc.NewTestClient(server.URL), slogt.New(t))
	assert.NilError(t, err)

	assert.NilError(t, p.InstanceStart(t.Context(), "web"))
	got, err := p.InstanceInspect(t.Context(), "web")
	assert.NilError(t, err)
	assert.Equal(t, got.Status, sablier.InstanceStatusStarting)
	assert.Equal(t, got.IsEnabled(), true)
}

// An unknown container ID resolves to a best-effort reference; reading it fails.
func TestProxmoxLXCProvider_InstanceInspect_UnknownNodeVMID(t *testing.T) {
	t.Parallel()

	server := proxmoxlxc.MockServer(t, []string{"pve1"}, []proxmoxlxc.TestContainer{
		{VMID: 100, Name: "web", Status: "running", Tags: "sablier", Node: "pve1"},
	})
	p, err := proxmoxlxc.New(t.Context(), proxmoxlxc.NewTestClient(server.URL), slogt.New(t))
	assert.NilError(t, err)

	_, err = p.InstanceInspect(t.Context(), "pve1/999")
	assert.ErrorContains(t, err, "999")
}

// An untagged container with the hostname of a tagged one must not be the
// container Sablier inspects, starts or stops. See #1179.
func TestProxmoxLXCProvider_InstanceInspect_TaggedWinsOverUntaggedHostname(t *testing.T) {
	t.Parallel()

	server := proxmoxlxc.MockServer(t, []string{"pve1"}, []proxmoxlxc.TestContainer{
		{VMID: 100, Name: "web", Status: "stopped", Tags: "sablier", Node: "pve1"},
		{VMID: 101, Name: "web", Status: "running", Tags: "", Node: "pve1"},
	})
	p, err := proxmoxlxc.New(t.Context(), proxmoxlxc.NewTestClient(server.URL), slogt.New(t))
	assert.NilError(t, err)

	got, err := p.InstanceInspect(t.Context(), "web")
	assert.NilError(t, err)
	assert.Equal(t, got.Status, sablier.InstanceStatusStopped)
}

// A hostname shared by two tagged containers is ambiguous: Sablier refuses it,
// and each container stays reachable by its ID.
func TestProxmoxLXCProvider_InstanceInspect_AmbiguousHostname(t *testing.T) {
	t.Parallel()

	server := proxmoxlxc.MockServer(t, []string{"pve1", "pve2"}, []proxmoxlxc.TestContainer{
		{VMID: 100, Name: "web", Status: "running", Tags: "sablier", Node: "pve1"},
		{VMID: 200, Name: "web", Status: "stopped", Tags: "sablier", Node: "pve2"},
	})
	p, err := proxmoxlxc.New(t.Context(), proxmoxlxc.NewTestClient(server.URL), slogt.New(t))
	assert.NilError(t, err)

	_, err = p.InstanceInspect(t.Context(), "web")
	assert.ErrorContains(t, err, "ambiguous")

	got, err := p.InstanceInspect(t.Context(), "pve2/200")
	assert.NilError(t, err)
	assert.Equal(t, got.Status, sablier.InstanceStatusStopped)
}
