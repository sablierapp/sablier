package kubernetes

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestSablierConfig(t *testing.T) {
	tests := []struct {
		name        string
		labels      map[string]string
		annotations map[string]string
		want        map[string]string
	}{
		{
			name:   "labels only",
			labels: map[string]string{"sablier.enable": "true", "sablier.group": "myapp"},
			want:   map[string]string{"sablier.enable": "true", "sablier.group": "myapp"},
		},
		{
			name:        "annotations add sablier keys invalid as labels",
			labels:      map[string]string{"sablier.enable": "true"},
			annotations: map[string]string{"sablier.running-days": "Mon,Tue,Wed", "sablier.running-hours": "09:00-18:00"},
			want: map[string]string{
				"sablier.enable":        "true",
				"sablier.running-days":  "Mon,Tue,Wed",
				"sablier.running-hours": "09:00-18:00",
			},
		},
		{
			name:        "annotations override labels",
			labels:      map[string]string{"sablier.enable": "true", "sablier.group": "from-label"},
			annotations: map[string]string{"sablier.group": "from-annotation"},
			want:        map[string]string{"sablier.enable": "true", "sablier.group": "from-annotation"},
		},
		{
			name:        "non sablier annotations are ignored",
			labels:      map[string]string{"sablier.enable": "true"},
			annotations: map[string]string{"kubectl.kubernetes.io/last-applied-configuration": "{...}", "app": "demo"},
			want:        map[string]string{"sablier.enable": "true"},
		},
		{
			name:        "nil maps",
			labels:      nil,
			annotations: nil,
			want:        map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sablierConfig(tt.labels, tt.annotations)
			assert.DeepEqual(t, got, tt.want)
		})
	}
}

