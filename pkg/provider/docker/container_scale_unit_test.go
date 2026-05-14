package docker

// Unit tests for the scale-mode resource parsing functions.
// These run without a real Docker daemon.

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestParseCPUNano(t *testing.T) {
	tests := []struct {
		name    string
		cpu     string
		want    int64
		wantErr bool
	}{
		{name: "half a core", cpu: "0.5", want: 500_000_000},
		{name: "one core", cpu: "1", want: 1_000_000_000},
		{name: "two cores", cpu: "2.0", want: 2_000_000_000},
		{name: "fractional", cpu: "0.25", want: 250_000_000},
		{name: "zero", cpu: "0", want: 0},
		{name: "invalid string", cpu: "bad", wantErr: true},
		{name: "negative", cpu: "-1", wantErr: true},
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

func TestParseMemoryBytes(t *testing.T) {
	tests := []struct {
		name    string
		memory  string
		want    int64
		wantErr bool
	}{
		{name: "128 megabytes lowercase", memory: "128m", want: 128 * 1024 * 1024},
		{name: "128 megabytes uppercase", memory: "128M", want: 128 * 1024 * 1024},
		{name: "1 gigabyte", memory: "1g", want: 1024 * 1024 * 1024},
		{name: "512 kilobytes", memory: "512k", want: 512 * 1024},
		{name: "raw bytes", memory: "1048576", want: 1048576},
		{name: "invalid", memory: "bad", wantErr: true},
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
