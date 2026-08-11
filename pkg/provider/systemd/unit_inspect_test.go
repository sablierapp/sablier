package systemd

import (
	"context"
	"testing"
	"time"

	"github.com/sablierapp/sablier/pkg/sablier"
	"gotest.tools/v3/assert"
)

func TestSystemdProvider_InstanceInspect(t *testing.T) {
	tests := []struct {
		name string
		unit mockUnitConfig
		want sablier.InstanceInfo
	}{
		{
			name: "active unit is ready",
			unit: mockUnitConfig{name: "web.service", active: true},
			want: sablier.InstanceInfo{
				Name:            "web.service",
				CurrentReplicas: 1,
				DesiredReplicas: 1,
				Status:          sablier.InstanceStatusReady,
				Provider:        sablier.ProviderSystemd,
				Config:          &sablier.InstanceConfig{},
			},
		},
		{
			name: "inactive unit is stopped",
			unit: mockUnitConfig{name: "web.service"},
			want: sablier.InstanceInfo{
				Name:            "web.service",
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          sablier.InstanceStatusStopped,
				Provider:        sablier.ProviderSystemd,
				Config:          &sablier.InstanceConfig{},
			},
		},
		{
			name: "deactivating unit is stopped",
			unit: mockUnitConfig{name: "web.service", status: "deactivating"},
			want: sablier.InstanceInfo{
				Name:            "web.service",
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          sablier.InstanceStatusStopped,
				Provider:        sablier.ProviderSystemd,
				Config:          &sablier.InstanceConfig{},
			},
		},
		{
			name: "activating unit is starting",
			unit: mockUnitConfig{name: "web.service", status: "activating"},
			want: sablier.InstanceInfo{
				Name:            "web.service",
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          sablier.InstanceStatusStarting,
				Provider:        sablier.ProviderSystemd,
				Config:          &sablier.InstanceConfig{},
			},
		},
		{
			name: "failed unit is in error",
			unit: mockUnitConfig{name: "web.service", status: "failed"},
			want: sablier.InstanceInfo{
				Name:            "web.service",
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          sablier.InstanceStatusError,
				Message:         "systemd unit failed",
				Provider:        sablier.ProviderSystemd,
				Config:          &sablier.InstanceConfig{},
			},
		},
		{
			name: "unknown status is unrecoverable",
			unit: mockUnitConfig{name: "web.service", status: "weird"},
			want: sablier.InstanceInfo{
				Name:            "web.service",
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          sablier.InstanceStatusError,
				Message:         `systemd unit status "weird" not handled`,
				Provider:        sablier.ProviderSystemd,
				Config:          &sablier.InstanceConfig{},
			},
		},
		{
			name: "enabled unit carries labels",
			unit: mockUnitConfig{
				name:         "web.service",
				active:       true,
				fragmentPath: writeUnitFile(t, "[X-Sablier]\nEnable=true\nGroup=team-a\n"),
			},
			want: sablier.InstanceInfo{
				Name:            "web.service",
				CurrentReplicas: 1,
				DesiredReplicas: 1,
				Status:          sablier.InstanceStatusReady,
				Provider:        sablier.ProviderSystemd,
				Enabled:         "true",
				Groups:          []string{"team-a"},
				Config:          &sablier.InstanceConfig{Enabled: true, Groups: []string{"team-a"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newMockSystemd(t, []mockUnitConfig{tt.unit})
			p := newProviderForTest(t, m, time.Second, nil)

			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			got, err := p.InstanceInspect(ctx, tt.unit.name)
			assert.NilError(t, err)
			assert.DeepEqual(t, got, tt.want)
		})
	}
}

func TestSystemdProvider_InstanceInspect_NotFound(t *testing.T) {
	m := newMockSystemd(t, nil)
	p := newProviderForTest(t, m, time.Second, nil)

	_, err := p.InstanceInspect(t.Context(), "missing.service")
	assert.ErrorContains(t, err, "not found")
}

func TestSystemdProvider_GetUnit_NotFoundLoadState(t *testing.T) {
	m := newMockSystemd(t, nil)
	p := newProviderForTest(t, m, time.Second, nil)

	_, err := p.getUnit(t.Context(), "missing.service")
	assert.ErrorContains(t, err, "not found")
}

func TestSystemdProvider_InstanceInspect_UnloadedUnitFile(t *testing.T) {
	m := newMockSystemd(t, []mockUnitConfig{{
		name:         "web.service",
		fragmentPath: writeUnitFile(t, "[X-Sablier]\nEnable=true\nGroup=team-a\n"),
	}})
	m.UnloadUnit("web.service")
	p := newProviderForTest(t, m, time.Second, nil)

	info, err := p.InstanceInspect(t.Context(), "web.service")
	assert.NilError(t, err)
	assert.Equal(t, info.Status, sablier.InstanceStatusStopped)
	assert.Equal(t, info.Enabled, "true")
	assert.DeepEqual(t, info.Groups, []string{"team-a"})
}
