package podman_test

import (
	"context"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/podman/v5/pkg/bindings/containers"
	"github.com/google/go-cmp/cmp"
	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider/podman"
	"github.com/sablierapp/sablier/pkg/sablier"
	"gotest.tools/v3/assert"
	"testing"
	"time"
)

func TestPodmanProvider_GetState(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	ctx := context.Background()
	type args struct {
		do func(pind *pindContainer) (string, error)
	}
	tests := []struct {
		name    string
		args    args
		want    sablier.InstanceInfo
		wantErr error
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
				Status:          sablier.InstanceStatusNotReady,
			},
			wantErr: nil,
		},
		{
			name: "running container without healthcheck",
			args: args{
				do: func(pind *pindContainer) (string, error) {
					c, err := pind.CreateMimic(ctx, MimicOptions{
						Healthcheck: nil,
					})
					if err != nil {
						return "", err
					}

					return c.ID, containers.Start(pind.connText, c.ID, nil)
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
						Cmd: []string{"-running", "-running-after", "1s", "-healthy", "true"},
						// Keep long interval so that the container is still in starting state
						Healthcheck: &manifest.Schema2HealthConfig{
							Test:          []string{"CMD", "/mimic", "healthcheck"},
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

					return c.ID, containers.Start(pind.connText, c.ID, nil)
				},
			},
			want: sablier.InstanceInfo{
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          sablier.InstanceStatusNotReady,
			},
			wantErr: nil,
		},
		{
			name: "running container with \"unhealthy\" health",
			args: args{
				do: func(pind *pindContainer) (string, error) {
					c, err := pind.CreateMimic(ctx, MimicOptions{
						Cmd: []string{"-running", "-running-after=1ms", "-healthy=false", "-healthy-after=1ms"},
						Healthcheck: &manifest.Schema2HealthConfig{
							Test:          []string{"CMD", "/mimic", "healthcheck"},
							Timeout:       time.Second,
							Interval:      time.Millisecond,
							StartInterval: time.Millisecond,
							StartPeriod:   time.Millisecond,
							Retries:       1,
						},
					})
					if err != nil {
						return "", err
					}

					err = containers.Start(pind.connText, c.ID, nil)
					if err != nil {
						return "", err
					}

					// Podman is not running with a healthcheck daemon, so we need to run the healthcheck manually
					_, err = containers.RunHealthCheck(pind.connText, c.ID, nil)
					if err != nil {
						return "", err
					}

					return c.ID, nil
				},
			},
			want: sablier.InstanceInfo{
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          sablier.InstanceStatusUnrecoverable,
				Message:         "container is unhealthy",
			},
			wantErr: nil,
		},
		{
			name: "running container with \"healthy\" health",
			args: args{
				do: func(pind *pindContainer) (string, error) {
					c, err := pind.CreateMimic(ctx, MimicOptions{
						Cmd: []string{"-running", "-running-after=1ms", "-healthy", "-healthy-after=1ms"},
						Healthcheck: &manifest.Schema2HealthConfig{
							Test:          []string{"CMD", "/mimic", "healthcheck"},
							Interval:      100 * time.Millisecond,
							Timeout:       time.Second,
							StartPeriod:   time.Second,
							StartInterval: 100 * time.Millisecond,
							Retries:       10,
						},
					})
					if err != nil {
						return "", err
					}

					err = containers.Start(pind.connText, c.ID, nil)
					if err != nil {
						return "", err
					}

					// Podman is not running with a healthcheck daemon, so we need to run the healthcheck manually
					_, err = containers.RunHealthCheck(pind.connText, c.ID, nil)
					if err != nil {
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
						Cmd: []string{"-running=false", "-exit-code=0"},
					})
					if err != nil {
						return "", err
					}

					err = containers.Start(pind.connText, c.ID, nil)
					if err != nil {
						return "", err
					}

					_, err = containers.Wait(pind.connText, c.ID, &containers.WaitOptions{
						Conditions: []string{"stopped"},
					})
					return c.ID, err
				},
			},
			want: sablier.InstanceInfo{
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          sablier.InstanceStatusNotReady,
			},
			wantErr: nil,
		},
		{
			name: "nginx exited container state with status code 137",
			args: args{
				do: func(pind *pindContainer) (string, error) {
					c, err := pind.CreateMimic(ctx, MimicOptions{
						Cmd: []string{"-running=false", "-exit-code=137"},
					})
					if err != nil {
						return "", err
					}

					err = containers.Start(pind.connText, c.ID, nil)
					if err != nil {
						return "", err
					}

					_, err = containers.Wait(pind.connText, c.ID, &containers.WaitOptions{
						Conditions: []string{"stopped"},
					})

					return c.ID, err
				},
			},
			want: sablier.InstanceInfo{
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          sablier.InstanceStatusUnrecoverable,
				Message:         "container exited with code \"137\"",
			},
			wantErr: nil,
		},
	}
	c := setupPinD(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p, err := podman.New(c.connText, slogt.New(t))

			name, err := tt.args.do(c)
			assert.NilError(t, err)

			tt.want.Name = name
			got, err := p.InstanceInspect(ctx, name)
			if !cmp.Equal(err, tt.wantErr) {
				t.Errorf("PodmanProvider.InstanceInspect() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.DeepEqual(t, got, tt.want)
		})
	}
}
