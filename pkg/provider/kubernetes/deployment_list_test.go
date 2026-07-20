package kubernetes

import (
	"testing"

	"gotest.tools/v3/assert"
	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestDeploymentToInstance_Delegated verifies that sablier.delegate-scaling is
// read from the merged label/annotation config onto InstanceConfiguration.Delegated,
// so the list path carries it without an extra inspect.
func TestDeploymentToInstance_Delegated(t *testing.T) {
	tests := []struct {
		name        string
		labels      map[string]string
		annotations map[string]string
		want        bool
	}{
		{
			name:   "absent means not delegated",
			labels: map[string]string{"sablier.enable": "true"},
			want:   false,
		},
		{
			name:   "set via label",
			labels: map[string]string{"sablier.enable": "true", "sablier.delegate-scaling": "true"},
			want:   true,
		},
		{
			name:        "set via annotation",
			labels:      map[string]string{"sablier.enable": "true"},
			annotations: map[string]string{"sablier.delegate-scaling": "true"},
			want:        true,
		},
		{
			name:        "annotation overrides label",
			labels:      map[string]string{"sablier.enable": "true", "sablier.delegate-scaling": "false"},
			annotations: map[string]string{"sablier.delegate-scaling": "true"},
			want:        true,
		},
		{
			name:   "invalid value is treated as false",
			labels: map[string]string{"sablier.enable": "true", "sablier.delegate-scaling": "maybe"},
			want:   false,
		},
	}

	p := &Provider{delimiter: "_"}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &v1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "app",
					Namespace:   "ns",
					Labels:      tt.labels,
					Annotations: tt.annotations,
				},
			}
			got := p.deploymentToInstance(d)
			assert.Equal(t, got.Delegated, tt.want)
		})
	}
}
