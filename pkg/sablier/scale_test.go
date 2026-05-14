package sablier_test

import (
	"testing"

	"github.com/sablierapp/sablier/pkg/sablier"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

func TestScaleConfigFromLabels_NoLabels(t *testing.T) {
	got := sablier.ScaleConfigFromLabels(map[string]string{})
	assert.Assert(t, got == nil)
}

func TestScaleConfigFromLabels_UnrelatedLabels(t *testing.T) {
	labels := map[string]string{
		"sablier.enable": "true",
		"sablier.group":  "mygroup",
	}
	got := sablier.ScaleConfigFromLabels(labels)
	assert.Assert(t, got == nil)
}

func TestScaleConfigFromLabels_IdleCPUOnly(t *testing.T) {
	labels := map[string]string{
		"sablier.idle.cpu": "0.1",
	}
	got := sablier.ScaleConfigFromLabels(labels)
	assert.Assert(t, cmp.DeepEqual(got, &sablier.ScaleConfig{
		Idle:   sablier.ResourceProfile{CPU: "0.1"},
		Active: sablier.ResourceProfile{},
	}))
}

func TestScaleConfigFromLabels_AllLabels(t *testing.T) {
	labels := map[string]string{
		"sablier.idle.cpu":    "0.1",
		"sablier.idle.memory": "128m",
		"sablier.active.cpu":  "2.0",
		"sablier.active.memory": "1g",
	}
	got := sablier.ScaleConfigFromLabels(labels)
	assert.Assert(t, cmp.DeepEqual(got, &sablier.ScaleConfig{
		Idle:   sablier.ResourceProfile{CPU: "0.1", Memory: "128m"},
		Active: sablier.ResourceProfile{CPU: "2.0", Memory: "1g"},
	}))
}

func TestPopulateEnabledAndGroup_WithScaleLabels(t *testing.T) {
	info := sablier.InstanceInfo{Name: "my-service"}
	labels := map[string]string{
		"sablier.enable":      "true",
		"sablier.group":       "backend",
		"sablier.idle.cpu":    "0.25",
		"sablier.idle.memory": "64m",
		"sablier.active.cpu":  "1.0",
	}

	sablier.PopulateEnabledAndGroup(&info, labels)

	assert.Equal(t, info.Enabled, "true")
	assert.Equal(t, info.Group, "backend")
	assert.Assert(t, info.ScaleConfig != nil, "ScaleConfig should be populated")
	assert.Equal(t, info.ScaleConfig.Idle.CPU, "0.25")
	assert.Equal(t, info.ScaleConfig.Idle.Memory, "64m")
	assert.Equal(t, info.ScaleConfig.Active.CPU, "1.0")
	assert.Equal(t, info.ScaleConfig.Active.Memory, "")
}

func TestPopulateEnabledAndGroup_NoScaleLabels(t *testing.T) {
	info := sablier.InstanceInfo{Name: "my-service"}
	labels := map[string]string{
		"sablier.enable": "true",
	}

	sablier.PopulateEnabledAndGroup(&info, labels)

	assert.Assert(t, info.ScaleConfig == nil, "ScaleConfig should be nil when no scale labels")
}
