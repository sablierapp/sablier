package dockerswarm_test

import (
	"context"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/google/go-cmp/cmp"
	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider/dockerswarm"
	"github.com/sablierapp/sablier/pkg/sablier"
	"gotest.tools/v3/assert"
	"testing"
	"time"
)

func TestDockerSwarmProvider_Stop(t *testing.T) {
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

					service, _, err := dind.client.ServiceInspectWithRaw(ctx, s.ID, types.ServiceInspectOptions{})
					if err != nil {
						return "", err
					}

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

					service, _, err := dind.client.ServiceInspectWithRaw(ctx, s.ID, types.ServiceInspectOptions{})
					if err != nil {
						return "", err
					}

					return service.Spec.Name, nil
				},
			},
			want: sablier.InstanceInfo{
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          sablier.InstanceStatusNotReady,
			},
			wantErr: nil,
		},
	}
	c := setupDinD(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p, err := dockerswarm.New(ctx, c.client, slogt.New(t))

			name, err := tt.args.do(c)
			assert.NilError(t, err)

			tt.want.Name = name
			err = p.InstanceStop(ctx, name)
			if !cmp.Equal(err, tt.wantErr) {
				t.Errorf("Provider.InstanceStop() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			service, _, err := c.client.ServiceInspectWithRaw(ctx, name, types.ServiceInspectOptions{})
			assert.NilError(t, err)
			assert.Equal(t, *service.Spec.Mode.Replicated.Replicas, uint64(0))
		})
	}
}
