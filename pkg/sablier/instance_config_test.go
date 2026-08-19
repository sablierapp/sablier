package sablier

import (
	"encoding/json"
	"testing"
	"time"

	"gotest.tools/v3/assert"
)

func TestInstanceConfigFromLabels(t *testing.T) {
	tests := []struct {
		name   string
		labels map[string]string
		want   InstanceConfig
	}{
		{
			name:   "no labels",
			labels: map[string]string{},
			want:   InstanceConfig{},
		},
		{
			name:   "enabled with default group",
			labels: map[string]string{LabelEnable: "true"},
			want:   InstanceConfig{Enabled: true, Groups: []string{"default"}},
		},
		{
			name:   "enable accepts only the exact string true",
			labels: map[string]string{LabelEnable: "1"},
			want:   InstanceConfig{},
		},
		{
			name:   "groups are not parsed for disabled instances",
			labels: map[string]string{LabelEnable: "false", LabelGroup: "team-a"},
			want:   InstanceConfig{},
		},
		{
			name: "full configuration",
			labels: map[string]string{
				LabelEnable:          "true",
				LabelGroup:           "team-a,team-b",
				LabelReadyAfter:      "30s",
				LabelReadyOnStart:    "true",
				LabelRunningHours:    "09:00-18:00",
				LabelRunningDays:     "Mon,Tue",
				LabelAntiAffinity:    "streaming",
				LabelDelegateScaling: "true",
			},
			want: InstanceConfig{
				Enabled:         true,
				Groups:          []string{"team-a", "team-b"},
				ReadyAfter:      30 * time.Second,
				ReadyOnStart:    true,
				RunningHours:    "09:00-18:00",
				RunningDays:     "Mon,Tue",
				AntiAffinity:    []string{"streaming"},
				DelegateScaling: true,
			},
		},
		{
			name:   "delegate-scaling parses a boolean",
			labels: map[string]string{LabelEnable: "true", LabelDelegateScaling: "true"},
			want:   InstanceConfig{Enabled: true, Groups: []string{"default"}, DelegateScaling: true},
		},
		{
			name:   "delegate-scaling false leaves scaling to Sablier",
			labels: map[string]string{LabelEnable: "true", LabelDelegateScaling: "false"},
			want:   InstanceConfig{Enabled: true, Groups: []string{"default"}},
		},
		{
			name: "invalid values are ignored, valid ones kept",
			labels: map[string]string{
				LabelEnable:          "true",
				LabelReadyAfter:      "soon",
				LabelReadyOnStart:    "yes-please",
				LabelRunningHours:    "9am-6pm",
				LabelRunningDays:     "Someday",
				LabelDelegateScaling: "maybe",
			},
			want: InstanceConfig{Enabled: true, Groups: []string{"default"}},
		},
		{
			name: "scale config only when a non-default scale label is present",
			labels: map[string]string{
				LabelEnable:       "true",
				LabelIdleReplicas: "1",
			},
			want: InstanceConfig{
				Enabled: true,
				Groups:  []string{"default"},
				Scale:   &ScaleConfig{Idle: ResourceProfile{Replicas: 1}, Active: ResourceProfile{Replicas: 1}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := InstanceConfigFromLabels(tt.labels, nil)
			assert.DeepEqual(t, tt.want, got)
		})
	}
}

func TestInstanceInfoIsEnabled(t *testing.T) {
	t.Run("parsed config wins", func(t *testing.T) {
		info := InstanceInfo{Enabled: "false", Config: &InstanceConfig{Enabled: true}}
		assert.Assert(t, info.IsEnabled())
	})
	t.Run("falls back to the flat field on old records", func(t *testing.T) {
		// A session record persisted by a version that predates Config.
		old := `{"name":"web","enabled":"true","status":"ready","currentReplicas":1,"desiredReplicas":1}`
		var info InstanceInfo
		assert.NilError(t, json.Unmarshal([]byte(old), &info))
		assert.Assert(t, info.Config == nil)
		assert.Assert(t, info.IsEnabled())
	})
	t.Run("disabled without config", func(t *testing.T) {
		assert.Assert(t, !InstanceInfo{Enabled: "1"}.IsEnabled())
		assert.Assert(t, !InstanceInfo{}.IsEnabled())
	})
	t.Run("list entries share the semantics", func(t *testing.T) {
		assert.Assert(t, InstanceConfiguration{Enabled: "true"}.IsEnabled())
		assert.Assert(t, !InstanceConfiguration{Enabled: "1"}.IsEnabled())
	})
}

func TestInstanceInfoIsDelegated(t *testing.T) {
	t.Run("parsed config drives the flag", func(t *testing.T) {
		assert.Assert(t, InstanceInfo{Config: &InstanceConfig{DelegateScaling: true}}.IsDelegated())
		assert.Assert(t, !InstanceInfo{Config: &InstanceConfig{DelegateScaling: false}}.IsDelegated())
	})
	t.Run("nil config is never delegated", func(t *testing.T) {
		assert.Assert(t, !InstanceInfo{}.IsDelegated())
	})
}

// TestPopulateEnabledAndGroup_WireCompat pins the compatibility contract of
// the first InstanceInfo split stage: the flat legacy fields keep their exact
// names and raw values on the wire, and the typed config is strictly
// additive under the "config" key.
func TestPopulateEnabledAndGroup_WireCompat(t *testing.T) {
	info := InstanceInfo{Name: "web"}
	PopulateEnabledAndGroup(&info, map[string]string{
		LabelEnable:       "true",
		LabelGroup:        "team-a",
		LabelRunningHours: "09:00-18:00",
	})

	raw, err := json.Marshal(info)
	assert.NilError(t, err)
	var m map[string]any
	assert.NilError(t, json.Unmarshal(raw, &m))

	// Legacy fields: identical names, raw string semantics preserved.
	assert.Equal(t, "true", m["enabled"], "legacy enabled must stay the raw label string")
	assert.DeepEqual(t, []any{"team-a"}, m["groups"])
	assert.Equal(t, "09:00-18:00", m["runningHours"])

	// Typed config: additive, under its own key, with the parsed boolean.
	cfg, ok := m["config"].(map[string]any)
	assert.Assert(t, ok, "config must be serialized additively")
	assert.Equal(t, true, cfg["enabled"])
}
