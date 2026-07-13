package kubernetes

import (
	"testing"

	"github.com/sablierapp/sablier/pkg/sablier"
)

func TestActiveReplicas(t *testing.T) {
	parsedWith := func(replicas int32) ParsedName {
		return ParsedName{Kind: "deployment", Namespace: "ns", Name: "app", Replicas: replicas}
	}

	tests := []struct {
		name     string
		labels   map[string]string
		parsed   ParsedName
		expected int32
	}{
		{
			name:     "no label, names replicas honored",
			labels:   map[string]string{},
			parsed:   parsedWith(2),
			expected: 2,
		},
		{
			name:     "no label, names replicas 1 keeps default",
			labels:   map[string]string{},
			parsed:   parsedWith(1),
			expected: 1,
		},
		{
			name:     "no label, names replicas 0 keeps default",
			labels:   map[string]string{},
			parsed:   parsedWith(0),
			expected: 1,
		},
		{
			name:     "label wins over names replicas",
			labels:   map[string]string{"sablier.active.replicas": "3"},
			parsed:   parsedWith(2),
			expected: 3,
		},
		{
			name:     "explicit label 1 wins over names replicas",
			labels:   map[string]string{"sablier.active.replicas": "1"},
			parsed:   parsedWith(5),
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc := sablier.ScaleConfigFromLabels(tt.labels)
			got := activeReplicas(sc, tt.labels, tt.parsed)
			if got != tt.expected {
				t.Errorf("activeReplicas() = %d, want %d", got, tt.expected)
			}
		})
	}
}
