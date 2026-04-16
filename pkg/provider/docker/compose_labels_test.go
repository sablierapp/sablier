package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseDependsOn(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected []DependsOnEntry
	}{
		{
			name:     "empty string",
			value:    "",
			expected: nil,
		},
		{
			name:  "single dependency",
			value: "db:service_started:true",
			expected: []DependsOnEntry{
				{Service: "db", Condition: "service_started", Restart: "true"},
			},
		},
		{
			name:  "multiple dependencies",
			value: "db:service_started:true,redis:service_healthy:false",
			expected: []DependsOnEntry{
				{Service: "db", Condition: "service_started", Restart: "true"},
				{Service: "redis", Condition: "service_healthy", Restart: "false"},
			},
		},
		{
			name:  "service name only",
			value: "db",
			expected: []DependsOnEntry{
				{Service: "db"},
			},
		},
		{
			name:  "service and condition only",
			value: "db:service_started",
			expected: []DependsOnEntry{
				{Service: "db", Condition: "service_started"},
			},
		},
		{
			name:  "trailing comma",
			value: "db:service_started:true,",
			expected: []DependsOnEntry{
				{Service: "db", Condition: "service_started", Restart: "true"},
			},
		},
		{
			name:  "whitespace handling",
			value: " db:service_started:true , redis:service_healthy:false ",
			expected: []DependsOnEntry{
				{Service: "db", Condition: "service_started", Restart: "true"},
				{Service: "redis", Condition: "service_healthy", Restart: "false"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseDependsOn(tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseComposeLabels(t *testing.T) {
	tests := []struct {
		name     string
		labels   map[string]string
		expected ComposeMetadata
	}{
		{
			name:     "no compose labels",
			labels:   map[string]string{"sablier.enable": "true"},
			expected: ComposeMetadata{},
		},
		{
			name: "project and service only",
			labels: map[string]string{
				"com.docker.compose.project": "myapp",
				"com.docker.compose.service": "web",
			},
			expected: ComposeMetadata{
				Project: "myapp",
				Service: "web",
			},
		},
		{
			name: "full compose labels",
			labels: map[string]string{
				"com.docker.compose.project":    "myapp",
				"com.docker.compose.service":    "api",
				"com.docker.compose.depends_on": "db:service_started:true,redis:service_healthy:false",
			},
			expected: ComposeMetadata{
				Project: "myapp",
				Service: "api",
				DependsOn: []DependsOnEntry{
					{Service: "db", Condition: "service_started", Restart: "true"},
					{Service: "redis", Condition: "service_healthy", Restart: "false"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseComposeLabels(tt.labels)
			assert.Equal(t, tt.expected, result)
		})
	}
}
