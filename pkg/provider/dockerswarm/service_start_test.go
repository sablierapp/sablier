package dockerswarm_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider/dockerswarm"
	"github.com/sablierapp/sablier/pkg/sablier"
	"gotest.tools/v3/assert"
)

func TestDockerSwarmProvider_Start(t *testing.T) {
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

					inspectResult, err := dind.client.ServiceInspect(ctx, s.ID, client.ServiceInspectOptions{})
					if err != nil {
						return "", err
					}
					service := inspectResult.Service

					return service.Spec.Name, err
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

					inspectResult, err := dind.client.ServiceInspect(ctx, s.ID, client.ServiceInspectOptions{})
					if err != nil {
						return "", err
					}
					service := inspectResult.Service

					return service.Spec.Name, nil
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
			name: "service with 0/0 replicas",
			args: args{
				do: func(dind *dindContainer) (string, error) {
					s, err := dind.CreateMimic(ctx, MimicOptions{})
					if err != nil {
						return "", err
					}

					inspectResult, err := dind.client.ServiceInspect(ctx, s.ID, client.ServiceInspectOptions{})
					if err != nil {
						return "", err
					}
					service := inspectResult.Service

					replicas := uint64(0)
					service.Spec.Mode.Replicated.Replicas = &replicas
					_, err = dind.client.ServiceUpdate(ctx, s.ID, client.ServiceUpdateOptions{Version: service.Version, Spec: service.Spec})
					if err != nil {
						return "", err
					}

					return service.Spec.Name, nil
				},
			},
			want: sablier.InstanceInfo{
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          sablier.InstanceStatusStopped,
			},
			wantErr: nil,
		},
	}
	c := sharedDinD
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p, err := dockerswarm.New(ctx, c.client, slogt.New(t), false)
			assert.NilError(t, err)

			name, err := tt.args.do(c)
			assert.NilError(t, err)
			t.Cleanup(func() {
				_, _ = sharedDinD.client.ServiceRemove(context.Background(), name, client.ServiceRemoveOptions{})
			})

			tt.want.Name = name
			err = p.InstanceStart(ctx, name)
			if !cmp.Equal(err, tt.wantErr) {
				t.Errorf("Provider.InstanceStop() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			service, err := c.client.ServiceInspect(ctx, name, client.ServiceInspectOptions{})
			assert.NilError(t, err)
			assert.Equal(t, *service.Service.Spec.Mode.Replicated.Replicas, uint64(1))
		})
	}
}
