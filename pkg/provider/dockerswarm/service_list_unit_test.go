package dockerswarm

import (
	"testing"

	"github.com/moby/moby/api/types/swarm"
	"github.com/sablierapp/sablier/pkg/sablier"
	"gotest.tools/v3/assert"
)

func TestServiceToInstance_AntiAffinity(t *testing.T) {
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
			name:   "disabled with anti-affinity",
			labels: map[string]string{sablier.LabelEnable: "false", sablier.LabelAntiAffinity: "streaming"},
		},
	}
	p := &Provider{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := swarm.Service{}
			s.Spec.Name = "nextcloud"
			s.Spec.Labels = tt.labels
			got := p.serviceToInstance(s)
			assert.DeepEqual(t, got.AntiAffinity, tt.want)
		})
	}
}
