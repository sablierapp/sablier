package sablier_test

import (
	"testing"

	"github.com/sablierapp/sablier/pkg/sablier"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

func TestScaleConfigFromLabels_NoLabels(t *testing.T) {
	got := sablier.ScaleConfigFromLabels(map[string]string{})
	assert.Assert(t, cmp.DeepEqual(got, sablier.ScaleConfig{
		Idle:   sablier.ResourceProfile{Replicas: 0},
		Active: sablier.ResourceProfile{Replicas: 1},
	}))
}

func TestScaleConfigFromLabels_UnrelatedLabels(t *testing.T) {
	labels := map[string]string{
		"sablier.enable": "true",
		"sablier.group":  "mygroup",
	}
	got := sablier.ScaleConfigFromLabels(labels)
	assert.Assert(t, cmp.DeepEqual(got, sablier.ScaleConfig{
		Idle:   sablier.ResourceProfile{Replicas: 0},
		Active: sablier.ResourceProfile{Replicas: 1},
	}))
}

func TestScaleConfigFromLabels_IdleCPUOnly(t *testing.T) {
	labels := map[string]string{
		"sablier.idle.cpu": "0.1",
	}
	got := sablier.ScaleConfigFromLabels(labels)
	assert.Assert(t, cmp.DeepEqual(got, sablier.ScaleConfig{
		Idle:   sablier.ResourceProfile{Replicas: 0, CPU: "0.1"},
		Active: sablier.ResourceProfile{Replicas: 1},
	}))
}

func TestScaleConfigFromLabels_AllLabels(t *testing.T) {
	labels := map[string]string{
		"sablier.idle.cpu":      "0.1",
		"sablier.idle.memory":   "128m",
		"sablier.active.cpu":    "2.0",
		"sablier.active.memory": "1g",
	}
	got := sablier.ScaleConfigFromLabels(labels)
	assert.Assert(t, cmp.DeepEqual(got, sablier.ScaleConfig{
		Idle:   sablier.ResourceProfile{Replicas: 0, CPU: "0.1", Memory: "128m"},
		Active: sablier.ResourceProfile{Replicas: 1, CPU: "2.0", Memory: "1g"},
	}))
}

func TestScaleConfigFromLabels_ReplicasOnly(t *testing.T) {
	labels := map[string]string{
		"sablier.idle.replicas": "1",
	}
	got := sablier.ScaleConfigFromLabels(labels)
	assert.Assert(t, cmp.DeepEqual(got, sablier.ScaleConfig{
		Idle:   sablier.ResourceProfile{Replicas: 1},
		Active: sablier.ResourceProfile{Replicas: 1},
	}))
}

func TestScaleConfigFromLabels_CustomReplicas(t *testing.T) {
	labels := map[string]string{
		"sablier.idle.replicas":   "2",
		"sablier.active.replicas": "4",
		"sablier.idle.cpu":        "0.1",
		"sablier.active.cpu":      "2.0",
	}
	got := sablier.ScaleConfigFromLabels(labels)
	assert.Assert(t, cmp.DeepEqual(got, sablier.ScaleConfig{
		Idle:   sablier.ResourceProfile{Replicas: 2, CPU: "0.1"},
		Active: sablier.ResourceProfile{Replicas: 4, CPU: "2.0"},
	}))
}

func TestScaleConfigFromLabels_ActiveReplicasOverride(t *testing.T) {
	labels := map[string]string{
		"sablier.idle.replicas":   "1",
		"sablier.active.replicas": "3",
	}
	got := sablier.ScaleConfigFromLabels(labels)
	assert.Assert(t, cmp.DeepEqual(got, sablier.ScaleConfig{
		Idle:   sablier.ResourceProfile{Replicas: 1},
		Active: sablier.ResourceProfile{Replicas: 3},
	}))
}

func TestPopulateEnabledAndGroup_WithScaleLabels(t *testing.T) {
	info := sablier.InstanceInfo{Name: "my-service"}
	labels := map[string]string{
		"sablier.enable":        "true",
		"sablier.group":         "backend",
		"sablier.idle.replicas": "1",
		"sablier.idle.cpu":      "0.25",
		"sablier.idle.memory":   "64m",
		"sablier.active.cpu":    "1.0",
	}

	sablier.PopulateEnabledAndGroup(&info, labels)

	assert.Equal(t, info.Enabled, "true")
	assert.DeepEqual(t, info.Groups, []string{"backend"})
	assert.Assert(t, info.ScaleConfig != nil, "ScaleConfig should be populated")
	assert.Equal(t, info.ScaleConfig.Idle.Replicas, int32(1))
	assert.Equal(t, info.ScaleConfig.Idle.CPU, "0.25")
	assert.Equal(t, info.ScaleConfig.Idle.Memory, "64m")
	assert.Equal(t, info.ScaleConfig.Active.Replicas, int32(1))
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
