package dockerswarm_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider/dockerswarm"
	"github.com/sablierapp/sablier/pkg/sablier"
	"gotest.tools/v3/assert"
)

func TestDockerSwarmProvider_GetState(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	t.Parallel()

	ctx := context.Background()
	type args struct {
		do func(dind *dindContainer) (string, error)
	}
	tests := []struct {
		name       string
		args       args
		want       sablier.InstanceInfo
		wantLabels map[string]string
		wantErr    error
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

					if err = WaitForServiceRunning(ctx, dind.client, service.Spec.Name, 1); err != nil {
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
		{
			name: "service with sablier labels",
			args: args{
				do: func(dind *dindContainer) (string, error) {
					s, err := dind.CreateMimic(ctx, MimicOptions{
						Cmd: []string{"/mimic"},
						Labels: map[string]string{
							"sablier.enable": "true",
							"sablier.group":  "myapp",
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

					if err = WaitForServiceRunning(ctx, dind.client, service.Spec.Name, 1); err != nil {
						return "", err
					}

					return service.Spec.Name, nil
				},
			},
			want: sablier.InstanceInfo{
				CurrentReplicas: 1,
				DesiredReplicas: 1,
				Status:          sablier.InstanceStatusReady,
				Enabled:         "true",
				Group:           "myapp",
			},
			wantLabels: map[string]string{
				"sablier.enable": "true",
				"sablier.group":  "myapp",
			},
			wantErr: nil,
		},
	}
	c := sharedDinD
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p, err := dockerswarm.New(ctx, c.client, slogt.New(t))
			assert.NilError(t, err)

			name, err := tt.args.do(c)
			assert.NilError(t, err)
			t.Cleanup(func() {
				_, _ = sharedDinD.client.ServiceRemove(context.Background(), name, client.ServiceRemoveOptions{})
			})

			tt.want.Name = name
			tt.want.Provider = "swarm"
			tt.want.Swarm = &sablier.SwarmServiceInfo{}
			got, err := p.InstanceInspect(ctx, name)
			if !cmp.Equal(err, tt.wantErr) {
				t.Errorf("Provider.InstanceInspect() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.DeepEqual(t, got, tt.want, cmpopts.IgnoreFields(sablier.SwarmServiceInfo{}, "ID", "Image", "Labels"))
			for k, v := range tt.wantLabels {
				if got.Swarm.Labels[k] != v {
					t.Errorf("Swarm.Labels[%q] = %q, want %q", k, got.Swarm.Labels[k], v)
				}
			}
		})
	}
}
