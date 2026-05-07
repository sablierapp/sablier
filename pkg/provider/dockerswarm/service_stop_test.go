package dockerswarm_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/swarm"
	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider/dockerswarm"
	"github.com/sablierapp/sablier/pkg/sablier"
	"gotest.tools/v3/assert"
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
		name            string
		args            args
		ignoreUnlabeled bool
		want            sablier.InstanceInfo
		wantErr         error
	}{
		{
			name: "unlabeled service stop is rejected when ignoreUnlabeled is enabled",
			args: args{
				do: func(dind *dindContainer) (string, error) {
					s, err := dind.CreateMimic(ctx, MimicOptions{})
					if err != nil {
						return "", err
					}

					service, _, err := dind.client.ServiceInspectWithRaw(ctx, s.ID, swarm.ServiceInspectOptions{})
					if err != nil {
						return "", err
					}

					return service.Spec.Name, nil
				},
			},
			ignoreUnlabeled: true,
			wantErr:         fmt.Errorf("is not managed by sablier"),
		},
		{
			name: "unlabeled service stop succeeds when ignoreUnlabeled is disabled",
			args: args{
				do: func(dind *dindContainer) (string, error) {
					s, err := dind.CreateMimic(ctx, MimicOptions{})
					if err != nil {
						return "", err
					}

					service, _, err := dind.client.ServiceInspectWithRaw(ctx, s.ID, swarm.ServiceInspectOptions{})
					if err != nil {
						return "", err
					}

					return service.Spec.Name, nil
				},
			},
			ignoreUnlabeled: false,
			wantErr:         nil,
		},
		{
			name: "labeled service stop succeeds from 1/1 replicas when ignoreUnlabeled is enabled",
			args: args{
				do: func(dind *dindContainer) (string, error) {
					s, err := dind.CreateMimic(ctx, MimicOptions{
						Cmd:         []string{"/mimic"},
						Healthcheck: nil,
						Labels:      map[string]string{"sablier.enable": "true"},
					})
					if err != nil {
						return "", err
					}

					service, _, err := dind.client.ServiceInspectWithRaw(ctx, s.ID, swarm.ServiceInspectOptions{})
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
			ignoreUnlabeled: true,
			wantErr:         nil,
		},
		{
			name: "labeled service stop succeeds from 0/1 replicas when ignoreUnlabeled is enabled",
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
						Labels: map[string]string{"sablier.enable": "true"},
					})
					if err != nil {
						return "", err
					}

					service, _, err := dind.client.ServiceInspectWithRaw(ctx, s.ID, swarm.ServiceInspectOptions{})
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
			ignoreUnlabeled: true,
			wantErr:         nil,
		},
	}
	c := setupDinD(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p, err := dockerswarm.New(ctx, c.client, slogt.New(t), tt.ignoreUnlabeled)
			assert.NilError(t, err)

			name, err := tt.args.do(c)
			assert.NilError(t, err)

			tt.want.Name = name
			err = p.InstanceStop(ctx, name)
			if tt.wantErr != nil {
				assert.ErrorContains(t, err, tt.wantErr.Error())
				return
			}
			assert.NilError(t, err)

			service, _, err := c.client.ServiceInspectWithRaw(ctx, name, swarm.ServiceInspectOptions{})
			assert.NilError(t, err)
			assert.Equal(t, *service.Spec.Mode.Replicated.Replicas, uint64(0))
		})
	}
}
