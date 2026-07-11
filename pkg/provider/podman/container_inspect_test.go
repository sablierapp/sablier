package podman_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider/podman"
	"github.com/sablierapp/sablier/pkg/sablier"
	"gotest.tools/v3/assert"
)

func TestPodmanProvider_GetState(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	t.Parallel()

	ctx := context.Background()
	type args struct {
		do func(pind *pindContainer) (string, error)
	}
	tests := []struct {
		name       string
		args       args
		want       sablier.InstanceInfo
		wantLabels map[string]string
		wantErr    error
	}{
		{
			name: "created container",
			args: args{
				do: func(pind *pindContainer) (string, error) {
					resp, err := pind.CreateMimic(ctx, MimicOptions{
						Cmd:         []string{"/mimic"},
						Healthcheck: nil,
					})
					return resp.ID, err
				},
			},
			want: sablier.InstanceInfo{
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          sablier.InstanceStatusStarting,
			},
			wantErr: nil,
		},
		{
			name: "running container without healthcheck",
			args: args{
				do: func(pind *pindContainer) (string, error) {
					c, err := pind.CreateMimic(ctx, MimicOptions{
						Cmd:         []string{"/mimic"},
						Healthcheck: nil,
					})
					if err != nil {
						return "", err
					}

					_, err = pind.client.ContainerStart(ctx, c.ID, client.ContainerStartOptions{})
					return c.ID, err
				},
			},
			want: sablier.InstanceInfo{
				CurrentReplicas: 1,
				DesiredReplicas: 1,
				Status:          sablier.InstanceStatusReady,
			},
			wantErr: nil,
		},
		{
			name: "running container with \"starting\" health",
			args: args{
				do: func(pind *pindContainer) (string, error) {
					c, err := pind.CreateMimic(ctx, MimicOptions{
						Cmd: []string{"/mimic", "-running", "-running-after", "1s", "-healthy", "true", "-port=82"},
						// Keep long interval so that the container is still in starting state
						Healthcheck: &container.HealthConfig{
							Test:          []string{"CMD", "/mimic", "healthcheck", "-port=82"},
							Interval:      time.Second,
							Timeout:       10 * time.Second,
							StartPeriod:   10 * time.Second,
							StartInterval: 10 * time.Second,
							Retries:       10,
						},
					})
					if err != nil {
						return "", err
					}

					_, err = pind.client.ContainerStart(ctx, c.ID, client.ContainerStartOptions{})
					return c.ID, err
				},
			},
			want: sablier.InstanceInfo{
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          sablier.InstanceStatusStarting,
			},
			wantErr: nil,
		},
		{
			name: "running container with \"unhealthy\" health",
			args: args{
				do: func(pind *pindContainer) (string, error) {
					c, err := pind.CreateMimic(ctx, MimicOptions{
						Cmd: []string{"/mimic", "-running", "-running-after=1ms", "-healthy=false", "-healthy-after=1ms", "-port=83"},
						Healthcheck: &container.HealthConfig{
							Test:        []string{"CMD", "/mimic", "healthcheck", "-port=83"},
							Timeout:     time.Second,
							Interval:    time.Second,
							StartPeriod: time.Second,
							Retries:     1,
						},
					})
					if err != nil {
						return "", err
					}

					_, err = pind.client.ContainerStart(ctx, c.ID, client.ContainerStartOptions{})
					if err != nil {
						return "", err
					}

					if err = WaitForContainerHealth(ctx, pind.client, c.ID, "unhealthy"); err != nil {
						return "", err
					}

					return c.ID, nil
				},
			},
			// A container that is running but not yet healthy is reported as
			// starting (Sablier keeps waiting for it to become healthy), with the
			// health state surfaced as a message — not an error. The podman
			// provider delegates inspection to the docker provider, so it shares
			// this behaviour.
			want: sablier.InstanceInfo{
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          sablier.InstanceStatusStarting,
				Message:         "container is running but not healthy yet",
			},
			wantErr: nil,
		},
		{
			name: "running container with \"healthy\" health",
			args: args{
				do: func(pind *pindContainer) (string, error) {
					c, err := pind.CreateMimic(ctx, MimicOptions{
						Cmd: []string{"/mimic", "-running", "-running-after=1ms", "-healthy", "-healthy-after=1ms", "-port=84"},
						Healthcheck: &container.HealthConfig{
							Test:        []string{"CMD", "/mimic", "healthcheck", "-port=84"},
							Interval:    time.Second,
							Timeout:     time.Second,
							StartPeriod: time.Second,
							Retries:     10,
						},
					})
					if err != nil {
						return "", err
					}

					_, err = pind.client.ContainerStart(ctx, c.ID, client.ContainerStartOptions{})
					if err != nil {
						return "", err
					}

					if err = WaitForContainerHealth(ctx, pind.client, c.ID, "healthy"); err != nil {
						return "", err
					}

					return c.ID, nil
				},
			},
			want: sablier.InstanceInfo{
				CurrentReplicas: 1,
				DesiredReplicas: 1,
				Status:          sablier.InstanceStatusReady,
			},
			wantErr: nil,
		},
		{
			name: "exited container with status code 0",
			args: args{
				do: func(pind *pindContainer) (string, error) {
					c, err := pind.CreateMimic(ctx, MimicOptions{
						Cmd: []string{"/mimic", "-running=false", "-exit-code=0"},
					})
					if err != nil {
						return "", err
					}

					_, err = pind.client.ContainerStart(ctx, c.ID, client.ContainerStartOptions{})
					if err != nil {
						return "", err
					}

					waitResult := pind.client.ContainerWait(ctx, c.ID, client.ContainerWaitOptions{
						Condition: container.WaitConditionNotRunning,
					})
					select {
					case <-waitResult.Result:
					case err = <-waitResult.Error:
						return "", err
					}
					return c.ID, nil
				},
			},
			// A container that exited successfully with no restart policy
			// explicitly defined keeps the historical behavior and is reported
			// as stopped. The podman provider delegates inspection to the
			// docker provider, so it shares this behaviour.
			want: sablier.InstanceInfo{
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          sablier.InstanceStatusStopped,
			},
			wantErr: nil,
		},
		{
			name: "exited container with status code 137",
			args: args{
				do: func(pind *pindContainer) (string, error) {
					c, err := pind.CreateMimic(ctx, MimicOptions{
						Cmd: []string{"/mimic", "-running=false", "-exit-code=137"},
					})
					if err != nil {
						return "", err
					}

					_, err = pind.client.ContainerStart(ctx, c.ID, client.ContainerStartOptions{})
					if err != nil {
						return "", err
					}

					waitResult := pind.client.ContainerWait(ctx, c.ID, client.ContainerWaitOptions{
						Condition: container.WaitConditionNotRunning,
					})
					select {
					case <-waitResult.Result:
					case err = <-waitResult.Error:
						return "", err
					}
					return c.ID, nil
				},
			},
			want: sablier.InstanceInfo{
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          sablier.InstanceStatusError,
				Message:         "container exited with code \"137\"",
			},
			wantErr: nil,
		},
		{
			name: "running container with sablier labels",
			args: args{
				do: func(pind *pindContainer) (string, error) {
					c, err := pind.CreateMimic(ctx, MimicOptions{
						Cmd: []string{"/mimic", "-running", "-running-after=1ms"},
						Labels: map[string]string{
							"sablier.enable":         "true",
							"sablier.group":          "myapp",
							"sablier.ready-on-start": "true",
						},
					})
					if err != nil {
						return "", err
					}
					_, err = pind.client.ContainerStart(ctx, c.ID, client.ContainerStartOptions{})
					return c.ID, err
				},
			},
			want: sablier.InstanceInfo{
				CurrentReplicas: 1,
				DesiredReplicas: 1,
				Status:          sablier.InstanceStatusReady,
				Enabled:         "true",
				Groups:          []string{"myapp"},
				ReadyOnStart:    true,
			},
			wantLabels: map[string]string{
				"sablier.enable":         "true",
				"sablier.group":          "myapp",
				"sablier.ready-on-start": "true",
			},
			wantErr: nil,
		},
	}
	c := sharedPinD
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p, err := podman.New(ctx, c.client, slogt.New(t))
			assert.NilError(t, err)

			name, err := tt.args.do(c)
			assert.NilError(t, err)

			tt.want.Name = name
			tt.want.Provider = "podman"
			tt.want.Podman = &sablier.PodmanContainerInfo{
				ID:    name,
				Image: "docker.io/sablierapp/mimic:v0.3.3",
			}
			// The provider mirrors the parsed label config into Config with the
			// same values as the flat fields each case already declares, so
			// derive the expectation instead of repeating it per case.
			tt.want.Config = &sablier.InstanceConfig{
				Enabled:      tt.want.Enabled == "true",
				Groups:       tt.want.Groups,
				ReadyAfter:   tt.want.ReadyAfter,
				ReadyOnStart: tt.want.ReadyOnStart,
				RunningHours: tt.want.RunningHours,
				RunningDays:  tt.want.RunningDays,
				AntiAffinity: tt.want.AntiAffinity,
				Scale:        tt.want.ScaleConfig,
			}
			got, err := p.InstanceInspect(ctx, name)
			if !cmp.Equal(err, tt.wantErr) {
				t.Errorf("PodmanProvider.InstanceInspect() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.DeepEqual(t, got, tt.want, cmpopts.IgnoreFields(sablier.PodmanContainerInfo{}, "Labels"))
			for k, v := range tt.wantLabels {
				if got.Podman.Labels[k] != v {
					t.Errorf("Podman.Labels[%q] = %q, want %q", k, got.Podman.Labels[k], v)
				}
			}
		})
	}
}
