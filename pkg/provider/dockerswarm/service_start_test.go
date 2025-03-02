package dockerswarm_test

import (
	"context"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/google/go-cmp/cmp"
	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/app/instance"
	"github.com/sablierapp/sablier/pkg/provider/dockerswarm"
	"gotest.tools/v3/assert"
	"testing"
	"time"
)

func TestDockerSwarmProvider_Start(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	ctx := context.Background()
	type args struct {
		do func(dind *dindContainer) (string, error)
	}
	tests := []struct {
		name    string
		args    args
		want    instance.State
		wantErr error
	}{
		{
			name: "service with 1/1 replicas",
			args: args{
				do: func(dind *dindContainer) (string, error) {
					s, err := dind.CreateMimic(ctx, MimicOptions{
						Cmd:         []string{"/mimic"},
						Healthcheck: nil,
					})
					if err != nil {
						return "", err
					}

					service, _, err := dind.client.ServiceInspectWithRaw(ctx, s.ID, types.ServiceInspectOptions{})
					if err != nil {
						return "", err
					}

					return service.Spec.Name, err
				},
			},
			want: instance.State{
				CurrentReplicas: 1,
				DesiredReplicas: 1,
				Status:          instance.Ready,
			},
			wantErr: nil,
		},
		{
			name: "service with 0/1 replicas",
			args: args{
				do: func(dind *dindContainer) (string, error) {
					s, err := dind.CreateMimic(ctx, MimicOptions{
						Cmd: []string{"/mimic", "-running-after=1ms", "-healthy=false", "-healthy-after=10s"},
						Healthcheck: &container.HealthConfig{
							Test:          []string{"CMD", "/mimic", "healthcheck"},
							Interval:      time.Second,
							Timeout:       time.Second,
							StartPeriod:   time.Second,
							StartInterval: time.Second,
							Retries:       10,
						},
					})
					if err != nil {
						return "", err
					}

					service, _, err := dind.client.ServiceInspectWithRaw(ctx, s.ID, types.ServiceInspectOptions{})
					if err != nil {
						return "", err
					}

					return service.Spec.Name, nil
				},
			},
			want: instance.State{
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          instance.NotReady,
			},
			wantErr: nil,
		},

		{
			name: "service with 0/0 replicas",
			args: args{
				do: func(dind *dindContainer) (string, error) {
					s, err := dind.CreateMimic(ctx, MimicOptions{})
					if err != nil {
						return "", err
					}

					service, _, err := dind.client.ServiceInspectWithRaw(ctx, s.ID, types.ServiceInspectOptions{})
					if err != nil {
						return "", err
					}

					replicas := uint64(0)
					service.Spec.Mode.Replicated.Replicas = &replicas
					_, err = dind.client.ServiceUpdate(ctx, s.ID, service.Version, service.Spec, types.ServiceUpdateOptions{})
					if err != nil {
						return "", err
					}

					return service.Spec.Name, nil
				},
			},
			want: instance.State{
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          instance.NotReady,
			},
			wantErr: nil,
		},
	}
	c := setupDinD(t, ctx)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p, err := dockerswarm.NewDockerSwarmProvider(ctx, c.client, slogt.New(t))

			name, err := tt.args.do(c)
			assert.NilError(t, err)

			tt.want.Name = name
			err = p.Start(ctx, name)
			if !cmp.Equal(err, tt.wantErr) {
				t.Errorf("DockerSwarmProvider.Stop() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			service, _, err := c.client.ServiceInspectWithRaw(ctx, name, types.ServiceInspectOptions{})
			assert.NilError(t, err)
			assert.Equal(t, *service.Spec.Mode.Replicated.Replicas, uint64(1))
		})
	}
}
