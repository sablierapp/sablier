package dockerswarm

// Unit tests for the scale-mode resource parsing functions in the Swarm provider.
// These run without a real Docker Swarm daemon.

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestSwarmParseCPUNano(t *testing.T) {
	tests := []struct {
		name    string
		cpu     string
		want    int64
		wantErr bool
	}{
		{name: "half a core", cpu: "0.5", want: 500_000_000},
		{name: "one core", cpu: "1", want: 1_000_000_000},
		{name: "two cores", cpu: "2.0", want: 2_000_000_000},
		{name: "zero", cpu: "0", want: 0},
		{name: "invalid", cpu: "notanumber", wantErr: true},
		{name: "negative", cpu: "-0.5", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCPUNano(tt.cpu)
			if tt.wantErr {
				assert.Assert(t, err != nil, "expected error for cpu=%q", tt.cpu)
				return
			}
			assert.NilError(t, err)
			assert.Equal(t, got, tt.want)
		})
	}
}

func TestSwarmParseMemoryBytes(t *testing.T) {
	tests := []struct {
		name    string
		memory  string
		want    int64
		wantErr bool
	}{
		{name: "256m", memory: "256m", want: 256 * 1024 * 1024},
		{name: "1g", memory: "1g", want: 1024 * 1024 * 1024},
		{name: "invalid", memory: "??", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseMemoryBytes(tt.memory)
			if tt.wantErr {
				assert.Assert(t, err != nil, "expected error for memory=%q", tt.memory)
				return
			}
			assert.NilError(t, err)
			assert.Equal(t, got, tt.want)
		})
	}
}
