package sablier

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveDependencyOrder(t *testing.T) {
	tests := []struct {
		name     string
		target   string
		deps     map[string][]string
		expected []string
		wantErr  bool
	}{
		{
			name:     "no dependencies",
			target:   "web",
			deps:     map[string][]string{},
			expected: []string{"web"},
		},
		{
			name:   "linear chain",
			target: "web",
			deps: map[string][]string{
				"web": {"api"},
				"api": {"db"},
			},
			expected: []string{"db", "api", "web"},
		},
		{
			name:   "diamond dependency",
			target: "web",
			deps: map[string][]string{
				"web":   {"api", "worker"},
				"api":   {"db"},
				"worker": {"db"},
			},
			expected: []string{"db", "api", "worker", "web"},
		},
		{
			name:   "cycle detection",
			target: "a",
			deps: map[string][]string{
				"a": {"b"},
				"b": {"c"},
				"c": {"a"},
			},
			wantErr: true,
		},
		{
			name:   "missing deps in map treated as leaf",
			target: "web",
			deps: map[string][]string{
				"web": {"db"},
			},
			expected: []string{"db", "web"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ResolveDependencyOrder(tt.target, tt.deps)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestResolveDependents(t *testing.T) {
	tests := []struct {
		name     string
		target   string
		deps     map[string][]string
		expected []string
	}{
		{
			name:     "no dependents",
			target:   "web",
			deps:     map[string][]string{},
			expected: nil,
		},
		{
			name:   "direct dependents",
			target: "db",
			deps: map[string][]string{
				"web": {"api"},
				"api": {"db"},
			},
			expected: []string{"api", "web"},
		},
		{
			name:   "multiple direct dependents",
			target: "db",
			deps: map[string][]string{
				"api":    {"db"},
				"worker": {"db"},
			},
			expected: []string{"api", "worker"},
		},
		{
			name:   "transitive dependents",
			target: "db",
			deps: map[string][]string{
				"web":    {"api"},
				"api":    {"db"},
				"worker": {"db"},
				"cron":   {"worker"},
			},
			expected: []string{"api", "worker", "web", "cron"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ResolveDependents(tt.target, tt.deps)
			assert.ElementsMatch(t, tt.expected, result)
		})
	}
}
