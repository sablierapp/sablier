package docker_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider/docker"
	"github.com/sablierapp/sablier/pkg/sablier"
	"gotest.tools/v3/assert"
)

func TestDockerClassicProvider_GetState(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	t.Parallel()

	ctx := context.Background()
	type args struct {
		do func(dind *dindContainer) (string, error)
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
				do: func(dind *dindContainer) (string, error) {
					resp, err := dind.CreateMimic(ctx, MimicOptions{
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
				do: func(dind *dindContainer) (string, error) {
					c, err := dind.CreateMimic(ctx, MimicOptions{
						Cmd:         []string{"/mimic"},
						Healthcheck: nil,
					})
					if err != nil {
						return "", err
					}

					_, err = dind.client.ContainerStart(ctx, c.ID, client.ContainerStartOptions{})
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
				do: func(dind *dindContainer) (string, error) {
					c, err := dind.CreateMimic(ctx, MimicOptions{
						Cmd: []string{"/mimic", "-running", "-running-after", "1s", "-healthy", "true"},
						// Keep long interval so that the container is still in starting state
						Healthcheck: &container.HealthConfig{
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

					_, err = dind.client.ContainerStart(ctx, c.ID, client.ContainerStartOptions{})
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
			name: "running container with \"unhealthy\" health",
			args: args{
				do: func(dind *dindContainer) (string, error) {
					c, err := dind.CreateMimic(ctx, MimicOptions{
						Cmd: []string{"/mimic", "-running", "-running-after=1ms", "-healthy=false", "-healthy-after=1ms"},
						Healthcheck: &container.HealthConfig{
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

					_, err = dind.client.ContainerStart(ctx, c.ID, client.ContainerStartOptions{})
					if err != nil {
						return "", err
					}

					if err = WaitForContainerHealth(ctx, dind.client, c.ID, "unhealthy"); err != nil {
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
				do: func(dind *dindContainer) (string, error) {
					c, err := dind.CreateMimic(ctx, MimicOptions{
						Cmd: []string{"/mimic", "-running", "-running-after=1ms", "-healthy", "-healthy-after=1ms"},
						Healthcheck: &container.HealthConfig{
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

					_, err = dind.client.ContainerStart(ctx, c.ID, client.ContainerStartOptions{})
					if err != nil {
						return "", err
					}

					if err = WaitForContainerHealth(ctx, dind.client, c.ID, "healthy"); err != nil {
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
			name: "paused container",
			args: args{
				do: func(dind *dindContainer) (string, error) {
					c, err := dind.CreateMimic(ctx, MimicOptions{})
					if err != nil {
						return "", err
					}

					_, err = dind.client.ContainerStart(ctx, c.ID, client.ContainerStartOptions{})
					if err != nil {
						return "", err
					}

					_, err = dind.client.ContainerPause(ctx, c.ID, client.ContainerPauseOptions{})
					if err != nil {
						return "", err
					}

					return c.ID, nil
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
			name: "exited container with status code 0",
			args: args{
				do: func(dind *dindContainer) (string, error) {
					c, err := dind.CreateMimic(ctx, MimicOptions{
						Cmd: []string{"/mimic", "-running=false", "-exit-code=0"},
					})
					if err != nil {
						return "", err
					}

					_, err = dind.client.ContainerStart(ctx, c.ID, client.ContainerStartOptions{})
					if err != nil {
						return "", err
					}

					wait := dind.client.ContainerWait(ctx, c.ID, client.ContainerWaitOptions{Condition: container.WaitConditionNotRunning})
					select {
					case err := <-wait.Error:
						return "", err
					case <-wait.Result:
					}

					return c.ID, nil
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
				do: func(dind *dindContainer) (string, error) {
					c, err := dind.CreateMimic(ctx, MimicOptions{
						Cmd: []string{"/mimic", "-running=false", "-exit-code=137"},
					})
					if err != nil {
						return "", err
					}

					_, err = dind.client.ContainerStart(ctx, c.ID, client.ContainerStartOptions{})
					if err != nil {
						return "", err
					}

					wait := dind.client.ContainerWait(ctx, c.ID, client.ContainerWaitOptions{Condition: container.WaitConditionNotRunning})
					select {
					case err := <-wait.Error:
						return "", err
					case <-wait.Result:
					}

					return c.ID, nil
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
	c := sharedDinD
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p, err := docker.New(ctx, c.client, slogt.New(t), "stop")
			assert.NilError(t, err)

			name, err := tt.args.do(c)
			assert.NilError(t, err)

			tt.want.Name = name
			got, err := p.InstanceInspect(ctx, name)
			if !cmp.Equal(err, tt.wantErr) {
				t.Errorf("DockerClassicProvider.InstanceInspect() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.DeepEqual(t, got, tt.want)
		})
	}
}
