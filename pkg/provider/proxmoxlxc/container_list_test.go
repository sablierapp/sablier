package proxmoxlxc

import (
	"sort"
	"testing"

	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
	"gotest.tools/v3/assert"
)

func TestProxmoxLXCProvider_InstanceList(t *testing.T) {
	t.Parallel()

	server := mockServer(t, []string{"pve1"}, []testContainer{
		{VMID: 100, Name: "web", Status: "running", Tags: "sablier;sablier-group-frontend", Node: "pve1"},
		{VMID: 101, Name: "db", Status: "stopped", Tags: "sablier", Node: "pve1"},
		{VMID: 102, Name: "unmanaged", Status: "running", Tags: "production", Node: "pve1"},
	})
	defer server.Close()

	p := newTestProvider(t, server.URL)
	instances, err := p.InstanceList(t.Context(), provider.InstanceListOptions{All: true})
	assert.NilError(t, err)

	sort.Slice(instances, func(i, j int) bool { return instances[i].Name < instances[j].Name })
	assert.DeepEqual(t, instances, []sablier.InstanceConfiguration{
		{Name: "db", Group: "default"},
		{Name: "web", Group: "frontend"},
	})
}

func TestProxmoxLXCProvider_InstanceList_RunningOnly(t *testing.T) {
	t.Parallel()

	server := proxmoxlxc.MockServer(t, []string{"pve1"}, []proxmoxlxc.TestContainer{
		{VMID: 100, Name: "web", Status: "running", Tags: "sablier;sablier-group-frontend", Node: "pve1"},
		{VMID: 101, Name: "db", Status: "stopped", Tags: "sablier", Node: "pve1"},
		{VMID: 102, Name: "unmanaged", Status: "running", Tags: "production", Node: "pve1"},
	})
	defer server.Close()

	p, err := proxmoxlxc.New(t.Context(), proxmoxlxc.NewTestClient(server.URL), slogt.New(t))
	assert.NilError(t, err)

	instances, err := p.InstanceList(t.Context(), provider.InstanceListOptions{All: false})
	assert.NilError(t, err)

	assert.DeepEqual(t, instances, []sablier.InstanceConfiguration{
		{Name: "web", Group: "frontend"},
	})
}

func TestProxmoxLXCProvider_InstanceList_MultiNode(t *testing.T) {
	t.Parallel()

	server := mockServer(t, []string{"pve1", "pve2"}, []testContainer{
		{VMID: 100, Name: "app1", Status: "running", Tags: "sablier;sablier-group-apps", Node: "pve1"},
		{VMID: 200, Name: "app2", Status: "stopped", Tags: "sablier;sablier-group-apps", Node: "pve2"},
	})
	defer server.Close()

	p := newTestProvider(t, server.URL)
	instances, err := p.InstanceList(t.Context(), provider.InstanceListOptions{All: true})
	assert.NilError(t, err)

	sort.Slice(instances, func(i, j int) bool { return instances[i].Name < instances[j].Name })
	assert.DeepEqual(t, instances, []sablier.InstanceConfiguration{
		{Name: "app1", Group: "apps"},
		{Name: "app2", Group: "apps"},
	})
}

func TestProxmoxLXCProvider_InstanceList_DuplicateHostname(t *testing.T) {
	t.Parallel()

	server := mockServer(t, []string{"pve1", "pve2"}, []testContainer{
		{VMID: 100, Name: "web", Status: "running", Tags: "sablier", Node: "pve1"},
		{VMID: 200, Name: "web", Status: "stopped", Tags: "sablier", Node: "pve2"},
	})
	defer server.Close()

	p := newTestProvider(t, server.URL)
	_, err := p.InstanceList(t.Context(), provider.InstanceListOptions{All: true})
	assert.ErrorContains(t, err, "duplicate hostname")
}

func TestProxmoxLXCProvider_InstanceGroups(t *testing.T) {
	t.Parallel()

	server := mockServer(t, []string{"pve1"}, []testContainer{
		{VMID: 100, Name: "web1", Status: "running", Tags: "sablier;sablier-group-frontend", Node: "pve1"},
		{VMID: 101, Name: "web2", Status: "running", Tags: "sablier;sablier-group-frontend", Node: "pve1"},
		{VMID: 102, Name: "db", Status: "stopped", Tags: "sablier", Node: "pve1"},
	})
	defer server.Close()

	p := newTestProvider(t, server.URL)
	groups, err := p.InstanceGroups(t.Context())
	assert.NilError(t, err)

	// Sort the slices for stable comparison
	for _, v := range groups {
		sort.Strings(v)
	}

	assert.DeepEqual(t, groups, map[string][]string{
		"frontend": {"web1", "web2"},
		"default":  {"db"},
	})
}
