package systemd

import (
	"context"
	"testing"
	"time"

	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
	"gotest.tools/v3/assert"
)

func TestSystemdProvider_InstanceList(t *testing.T) {
	m := newMockSystemd(t, []mockUnitConfig{
		{
			name:         "web.service",
			active:       true,
			fragmentPath: writeUnitFile(t, "[X-Sablier]\nEnable=true\nGroup=team-a,team-b\n"),
		},
		{
			name:         "db.service",
			fragmentPath: writeUnitFile(t, "[X-Sablier]\nEnable=true\n"),
		},
		{
			name:         "jobs.service",
			active:       true,
			fragmentPath: writeUnitFile(t, "[X-Sablier]\nEnable=false\n"),
		},
	})
	p := newProviderForTest(t, m, time.Second, nil)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	active, err := p.InstanceList(ctx, provider.InstanceListOptions{})
	assert.NilError(t, err)
	assert.DeepEqual(t, active, []sablier.InstanceConfiguration{
		{Name: "web.service", Groups: []string{"team-a", "team-b"}, Enabled: "true"},
	})

	all, err := p.InstanceList(ctx, provider.InstanceListOptions{All: true})
	assert.NilError(t, err)
	assert.DeepEqual(t, all, []sablier.InstanceConfiguration{
		{Name: "db.service", Groups: []string{"default"}, Enabled: "true"},
		{Name: "web.service", Groups: []string{"team-a", "team-b"}, Enabled: "true"},
	})
}

func TestSystemdProvider_InstanceGroups(t *testing.T) {
	m := newMockSystemd(t, []mockUnitConfig{
		{
			name:         "web.service",
			fragmentPath: writeUnitFile(t, "[X-Sablier]\nEnable=true\nGroup=team-a,team-b\n"),
		},
		{
			name:         "db.service",
			fragmentPath: writeUnitFile(t, "[X-Sablier]\nEnable=true\n"),
		},
		{
			name:         "jobs.service",
			fragmentPath: writeUnitFile(t, "[X-Sablier]\nEnable=false\nGroup=team-a\n"),
		},
	})
	p := newProviderForTest(t, m, time.Second, nil)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	groups, err := p.InstanceGroups(ctx)
	assert.NilError(t, err)
	assert.DeepEqual(t, groups, map[string][]string{
		"team-a":  {"web.service"},
		"team-b":  {"web.service"},
		"default": {"db.service"},
	})
}

func TestSystemdProvider_InstanceList_AllIncludesUnloadedUnitFile(t *testing.T) {
	path := writeUnitFile(t, "[X-Sablier]\nEnable=true\nGroup=team-a\n")
	m := newMockSystemd(t, []mockUnitConfig{{name: "web.service", fragmentPath: path}})
	m.UnloadUnit("web.service")
	p := newProviderForTest(t, m, time.Second, nil)

	instances, err := p.InstanceList(t.Context(), provider.InstanceListOptions{All: true})
	assert.NilError(t, err)
	assert.DeepEqual(t, instances, []sablier.InstanceConfiguration{
		{Name: "web.service", Groups: []string{"team-a"}, Enabled: "true"},
	})
}

func TestUnitToInstance(t *testing.T) {
	tests := []struct {
		name string
		unit Unit
		want sablier.InstanceConfiguration
	}{
		{
			name: "no labels",
			unit: Unit{status: dbus.UnitStatus{Name: "web.service"}, labels: nil},
			want: sablier.InstanceConfiguration{Name: "web.service"},
		},
		{
			name: "enabled with groups",
			unit: Unit{status: dbus.UnitStatus{Name: "web.service"}, labels: map[string]string{sablier.LabelEnable: "true", sablier.LabelGroup: "team-a,team-b"}},
			want: sablier.InstanceConfiguration{Name: "web.service", Groups: []string{"team-a", "team-b"}, Enabled: "true"},
		},
		{
			name: "disabled ignores groups",
			unit: Unit{status: dbus.UnitStatus{Name: "web.service"}, labels: map[string]string{sablier.LabelEnable: "false", sablier.LabelGroup: "team-a"}},
			want: sablier.InstanceConfiguration{Name: "web.service", Enabled: "false"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.DeepEqual(t, unitToInstance(tt.unit), tt.want)
		})
	}
}
