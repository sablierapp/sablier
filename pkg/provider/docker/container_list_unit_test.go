package docker

// Unit tests for converting a listed container to an InstanceConfiguration.
// These run without a real Docker daemon.

import (
	"testing"

	"github.com/moby/moby/api/types/container"
	"github.com/sablierapp/sablier/pkg/sablier"
	"gotest.tools/v3/assert"
)

func TestContainerToInstance_AntiAffinity(t *testing.T) {
	tests := []struct {
		name   string
		labels map[string]string
		want   []string
	}{
		{
			name:   "enabled with anti-affinity",
			labels: map[string]string{sablier.LabelEnable: "true", sablier.LabelAntiAffinity: "streaming, gaming,streaming"},
			want:   []string{"streaming", "gaming"},
		},
		{
			name:   "enabled without anti-affinity",
			labels: map[string]string{sablier.LabelEnable: "true"},
		},
		{
			// Same as the inspect path (sablier.InstanceConfigFromLabels).
			name:   "disabled with anti-affinity",
			labels: map[string]string{sablier.LabelEnable: "false", sablier.LabelAntiAffinity: "streaming"},
			want:   []string{"streaming"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containerToInstance(container.Summary{Names: []string{"/nextcloud"}, Labels: tt.labels})
			assert.DeepEqual(t, got.AntiAffinity, tt.want)
		})
	}
}
