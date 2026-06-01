package docker

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"github.com/sablierapp/sablier/pkg/sablier"
)

// fakeInspectClient is a minimal client.APIClient that only implements
// ContainerInspect. All other methods are inherited from the embedded interface
// and will panic if called, which is fine because the functions under test only
// rely on ContainerInspect.
type fakeInspectClient struct {
	client.APIClient
	result client.ContainerInspectResult
	err    error
}

func (f fakeInspectClient) ContainerInspect(context.Context, string, client.ContainerInspectOptions) (client.ContainerInspectResult, error) {
	return f.result, f.err
}

func newTestProvider(c client.APIClient) *Provider {
	return &Provider{
		Client: c,
		l:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func inspectResult(state *container.State, hc *container.HostConfig) client.ContainerInspectResult {
	return client.ContainerInspectResult{
		Container: container.InspectResponse{
			State:      state,
			HostConfig: hc,
		},
	}
}

func TestCheckDependencyCondition(t *testing.T) {
	hcNo := &container.HostConfig{RestartPolicy: container.RestartPolicy{Name: container.RestartPolicyDisabled}}
	hcAlways := &container.HostConfig{RestartPolicy: container.RestartPolicy{Name: container.RestartPolicyAlways}}

	tests := []struct {
		name      string
		condition string
		state     *container.State
		hc        *container.HostConfig
		want      bool
		wantErr   bool
	}{
		{
			name:      "completed successfully: still running",
			condition: conditionServiceCompletedSuccessfully,
			state:     &container.State{Status: container.StateRunning, Running: true},
			hc:        hcNo,
			want:      false,
		},
		{
			name:      "completed successfully: exited zero, no restart",
			condition: conditionServiceCompletedSuccessfully,
			state:     &container.State{Status: container.StateExited, ExitCode: 0},
			hc:        hcNo,
			want:      true,
		},
		{
			name:      "completed successfully: exited non-zero is an error",
			condition: conditionServiceCompletedSuccessfully,
			state:     &container.State{Status: container.StateExited, ExitCode: 1},
			hc:        hcNo,
			wantErr:   true,
		},
		{
			name:      "completed successfully: exited zero but restarts is transient",
			condition: conditionServiceCompletedSuccessfully,
			state:     &container.State{Status: container.StateExited, ExitCode: 0},
			hc:        hcAlways,
			want:      false,
		},
		{
			name:      "healthy: running without healthcheck fails fast",
			condition: conditionServiceHealthy,
			state:     &container.State{Status: container.StateRunning, Running: true},
			hc:        hcNo,
			wantErr:   true,
		},
		{
			name:      "healthy: running and healthy",
			condition: conditionServiceHealthy,
			state:     &container.State{Status: container.StateRunning, Running: true, Health: &container.Health{Status: container.Healthy}},
			hc:        hcNo,
			want:      true,
		},
		{
			name:      "healthy: running and unhealthy",
			condition: conditionServiceHealthy,
			state:     &container.State{Status: container.StateRunning, Running: true, Health: &container.Health{Status: container.Unhealthy}},
			hc:        hcNo,
			want:      false,
		},
		{
			name:      "running or healthy: running without healthcheck",
			condition: conditionServiceRunningOrHealthy,
			state:     &container.State{Status: container.StateRunning, Running: true},
			hc:        hcNo,
			want:      true,
		},
		{
			name:      "started: running",
			condition: conditionServiceStarted,
			state:     &container.State{Status: container.StateRunning, Running: true},
			hc:        hcNo,
			want:      true,
		},
		{
			name:      "empty condition defaults to started",
			condition: "",
			state:     &container.State{Status: container.StateExited, Running: false},
			hc:        hcNo,
			want:      false,
		},
		{
			name:      "unknown condition falls back to started",
			condition: "service_unknown",
			state:     &container.State{Status: container.StateRunning, Running: true},
			hc:        hcNo,
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newTestProvider(fakeInspectClient{result: inspectResult(tt.state, tt.hc)})
			got, err := p.checkDependencyCondition(context.Background(), "dep", tt.condition)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheckDependencyConditionInspectError(t *testing.T) {
	p := newTestProvider(fakeInspectClient{err: errors.New("boom")})
	_, err := p.checkDependencyCondition(context.Background(), "dep", conditionServiceStarted)
	if err == nil {
		t.Fatalf("expected error when inspect fails")
	}
}

func TestIsHealthy(t *testing.T) {
	tests := []struct {
		name            string
		state           *container.State
		fallbackRunning bool
		want            bool
	}{
		{name: "nil state", state: nil, want: false},
		{name: "not running", state: &container.State{Running: false}, want: false},
		{name: "running no healthcheck fallback true", state: &container.State{Running: true}, fallbackRunning: true, want: true},
		{name: "running no healthcheck fallback false", state: &container.State{Running: true}, fallbackRunning: false, want: false},
		{name: "running healthy", state: &container.State{Running: true, Health: &container.Health{Status: container.Healthy}}, want: true},
		{name: "running unhealthy", state: &container.State{Running: true, Health: &container.Health{Status: container.Unhealthy}}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isHealthy(tt.state, tt.fallbackRunning); got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRestartPolicyMode(t *testing.T) {
	if got := restartPolicyMode(nil); got != container.RestartPolicyDisabled {
		t.Fatalf("nil host config: got %q, want %q", got, container.RestartPolicyDisabled)
	}
	hc := &container.HostConfig{RestartPolicy: container.RestartPolicy{Name: container.RestartPolicyAlways}}
	if got := restartPolicyMode(hc); got != container.RestartPolicyAlways {
		t.Fatalf("got %q, want %q", got, container.RestartPolicyAlways)
	}
}

func TestRestartsOnSuccess(t *testing.T) {
	tests := []struct {
		mode container.RestartPolicyMode
		want bool
	}{
		{container.RestartPolicyAlways, true},
		{container.RestartPolicyUnlessStopped, true},
		{container.RestartPolicyOnFailure, false},
		{container.RestartPolicyDisabled, false},
	}
	for _, tt := range tests {
		if got := restartsOnSuccess(tt.mode); got != tt.want {
			t.Fatalf("mode %q: got %v, want %v", tt.mode, got, tt.want)
		}
	}
}

func TestHealthStatus(t *testing.T) {
	if got := healthStatus(nil); got != "no healthcheck defined" {
		t.Fatalf("nil health: got %q", got)
	}
	if got := healthStatus(&container.Health{Status: container.Healthy}); got != string(container.Healthy) {
		t.Fatalf("got %q, want %q", got, container.Healthy)
	}
}

func TestInstanceInspectExitedRestartPolicy(t *testing.T) {
	ctx := context.Background()

	newResult := func(mode container.RestartPolicyMode) client.ContainerInspectResult {
		return client.ContainerInspectResult{
			Container: container.InspectResponse{
				ID:         "abc",
				State:      &container.State{Status: container.StateExited, ExitCode: 0},
				HostConfig: &container.HostConfig{RestartPolicy: container.RestartPolicy{Name: mode}},
				Config:     &container.Config{Image: "img"},
			},
		}
	}

	tests := []struct {
		name  string
		honor bool
		mode  container.RestartPolicyMode
		want  sablier.InstanceStatus
	}{
		// HonorRestartPolicy disabled (default): always stopped, regardless of policy.
		{"disabled: no -> stopped", false, container.RestartPolicyDisabled, sablier.InstanceStatusStopped},
		{"disabled: on-failure -> stopped", false, container.RestartPolicyOnFailure, sablier.InstanceStatusStopped},
		{"disabled: always -> stopped", false, container.RestartPolicyAlways, sablier.InstanceStatusStopped},
		{"disabled: unless-stopped -> stopped", false, container.RestartPolicyUnlessStopped, sablier.InstanceStatusStopped},
		// HonorRestartPolicy enabled: honor the policy.
		{"honor: no -> ready", true, container.RestartPolicyDisabled, sablier.InstanceStatusReady},
		{"honor: on-failure -> ready", true, container.RestartPolicyOnFailure, sablier.InstanceStatusReady},
		{"honor: always -> starting", true, container.RestartPolicyAlways, sablier.InstanceStatusStarting},
		{"honor: unless-stopped -> starting", true, container.RestartPolicyUnlessStopped, sablier.InstanceStatusStarting},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newTestProvider(fakeInspectClient{result: newResult(tt.mode)})
			p.HonorRestartPolicy = tt.honor
			got, err := p.InstanceInspect(ctx, "abc")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Status != tt.want {
				t.Fatalf("got %q, want %q", got.Status, tt.want)
			}
		})
	}
}
