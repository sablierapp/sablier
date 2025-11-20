package digitalocean_test

import (
	"context"
	"testing"

	"github.com/sablierapp/sablier/pkg/provider"
	"gotest.tools/v3/assert"
)

func TestDigitalOceanProvider_InstanceList(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	p, _ := setupDigitalOcean(t)

	tests := []struct {
		name    string
		options provider.InstanceListOptions
	}{
		{
			name: "list all apps",
			options: provider.InstanceListOptions{
				All: true,
			},
		},
		{
			name: "list only sablier-enabled apps",
			options: provider.InstanceListOptions{
				All: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instances, err := p.InstanceList(ctx, tt.options)
			assert.NilError(t, err)

			t.Logf("Found %d apps", len(instances))
			for _, instance := range instances {
				t.Logf("App: %s, Group: %s", instance.Name, instance.Group)
			}

			// If we're filtering, verify all instances have proper configuration
			if !tt.options.All {
				for _, instance := range instances {
					assert.Assert(t, instance.Name != "", "instance name should not be empty")
					assert.Assert(t, instance.Group != "", "instance group should not be empty")
				}
			}
		})
	}
}

func TestDigitalOceanProvider_InstanceGroups(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	p, _ := setupDigitalOcean(t)

	groups, err := p.InstanceGroups(ctx)
	assert.NilError(t, err)

	t.Logf("Found %d groups", len(groups))
	for groupName, appIDs := range groups {
		t.Logf("Group '%s' has %d apps: %v", groupName, len(appIDs), appIDs)
	}

	// Verify groups structure
	for groupName, appIDs := range groups {
		assert.Assert(t, groupName != "", "group name should not be empty")
		assert.Assert(t, len(appIDs) > 0, "group should have at least one app")
	}
}
