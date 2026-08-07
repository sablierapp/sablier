package docker

// Unit tests for the scale-mode resource parsing functions.
// These run without a real Docker daemon.

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/moby/moby/client"
	"github.com/sablierapp/sablier/pkg/sablier"
	"gotest.tools/v3/assert"
)

// stubUpdateClient implements just enough of client.APIClient to let
// applyResources call ContainerUpdate without a real daemon. All other methods
// are inherited from the embedded (nil) interface and must not be called.
type stubUpdateClient struct {
	client.APIClient
}

func (stubUpdateClient) ContainerUpdate(context.Context, string, client.ContainerUpdateOptions) (client.ContainerUpdateResult, error) {
	return client.ContainerUpdateResult{}, nil
}

// TestApplyResources_BlkioDeviceVersionWarning verifies that a warning is emitted
// only when per-device blkio limits are requested against a daemon older than the
// minimum supported API version.
func TestApplyResources_BlkioDeviceVersionWarning(t *testing.T) {
	const warning = "per-device blkio throttling requires a newer Docker daemon"

	deviceProfile := sablier.ResourceProfile{
		BlkioDeviceReadBps: []sablier.BlkioThrottleDevice{{Path: "/dev/sda", Rate: "5m"}},
	}
	weightOnlyProfile := sablier.ResourceProfile{BlkioWeight: 100}

	tests := []struct {
		name       string
		apiVersion string
		profile    sablier.ResourceProfile
		wantWarn   bool
	}{
		{name: "old daemon with device limits warns", apiVersion: "1.51", profile: deviceProfile, wantWarn: true},
		{name: "exact min version does not warn", apiVersion: "1.55", profile: deviceProfile, wantWarn: false},
		{name: "newer daemon does not warn", apiVersion: "1.60", profile: deviceProfile, wantWarn: false},
		{name: "unknown version does not warn", apiVersion: "", profile: deviceProfile, wantWarn: false},
		{name: "global blkio-weight on old daemon does not warn", apiVersion: "1.51", profile: weightOnlyProfile, wantWarn: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			p := &Provider{
				Client:     stubUpdateClient{},
				l:          slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})),
				apiVersion: tt.apiVersion,
			}
			err := p.applyResources(context.Background(), "c1", tt.profile)
			assert.NilError(t, err)
			warned := strings.Contains(buf.String(), warning)
			assert.Equal(t, warned, tt.wantWarn, "logs: %s", buf.String())
		})
	}
}

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

func TestParseBpsRate(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    uint64
		wantErr bool
	}{
		{name: "10 megabytes/s lowercase", input: "10m", want: 10 * 1024 * 1024},
		{name: "10 megabytes/s uppercase", input: "10M", want: 10 * 1024 * 1024},
		{name: "100 kilobytes/s", input: "100k", want: 100 * 1024},
		{name: "1 gigabyte/s", input: "1g", want: 1024 * 1024 * 1024},
		{name: "raw bytes", input: "1048576", want: 1048576},
		{name: "invalid string", input: "fast", wantErr: true},
		{name: "empty", input: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseBpsRate(tt.input)
			if tt.wantErr {
				assert.Assert(t, err != nil, "expected error for input=%q", tt.input)
				return
			}
			assert.NilError(t, err)
			assert.Equal(t, got, tt.want)
		})
	}
}

func TestParseIOpsRate(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    uint64
		wantErr bool
	}{
		{name: "100 iops", input: "100", want: 100},
		{name: "zero", input: "0", want: 0},
		{name: "large value", input: "10000", want: 10000},
		{name: "invalid string", input: "many", wantErr: true},
		{name: "negative", input: "-1", wantErr: true},
		{name: "float", input: "50.5", wantErr: true},
		{name: "empty", input: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseIOpsRate(tt.input)
			if tt.wantErr {
				assert.Assert(t, err != nil, "expected error for input=%q", tt.input)
				return
			}
			assert.NilError(t, err)
			assert.Equal(t, got, tt.want)
		})
	}
}
