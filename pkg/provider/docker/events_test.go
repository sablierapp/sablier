package docker_test

import (
	"context"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/rs/zerolog"
	"github.com/sablierapp/sablier/pkg/provider/docker"
	"github.com/sablierapp/sablier/pkg/sablier"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestDockerProvider_Events(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	type args struct {
		do   func(dind *dindContainer, provider *docker.DockerProvider) error
		name string
	}
	tests := []struct {
		name       string
		args       args
		wantEvents []sablier.Message
		wantErr    assert.ErrorAssertionFunc
	}{
		{
			name: "container full lifecycle",
			args: args{
				do: func(dind *dindContainer, provider *docker.DockerProvider) error {
					created := provider.AfterAction(ctx, "test-events", events.ActionCreate)
					mimic, err := dind.CreateMimic(ctx, MimicOptions{
						Name:         "test-events",
						Registered:   true,
						SablierGroup: "test",
						WithHealth:   true,
					})
					if err != nil {
						return err
					}

					<-created

					// Started + waiting for health-checks  if any
					ready := provider.AfterReady(ctx, mimic.ID, 0)
					err = dind.client.ContainerStart(ctx, mimic.ID, container.StartOptions{})
					if err != nil {
						return err
					}

					<-ready

					stopped := provider.AfterAction(ctx, "test-events", events.ActionStop)
					err = dind.client.ContainerStop(ctx, mimic.ID, container.StopOptions{})
					if err != nil {
						return err
					}
					<-stopped

					removed := provider.AfterAction(ctx, "test-events", events.ActionDestroy)
					err = dind.client.ContainerRemove(ctx, mimic.ID, container.RemoveOptions{})
					if err != nil {
						return err
					}
					<-removed
					return nil
				},
				name: "test-events",
			},
			wantEvents: []sablier.Message{
				{
					Action: sablier.EventActionCreate,
					Instance: sablier.InstanceConfig{
						Name:            "test-events",
						Group:           "test",
						DesiredReplicas: 1,
						Enabled:         true,
					},
				},
				{
					Action: sablier.EventActionStart,
					Instance: sablier.InstanceConfig{
						Name:            "test-events",
						Group:           "test",
						DesiredReplicas: 1,
						Enabled:         true,
					},
				}, {
					Action: sablier.EventActionStop,
					Instance: sablier.InstanceConfig{
						Name:            "test-events",
						Group:           "test",
						DesiredReplicas: 1,
						Enabled:         true,
					},
				},
				{
					Action: sablier.EventActionRemove,
					Instance: sablier.InstanceConfig{
						Name:            "test-events",
						Group:           "test",
						DesiredReplicas: 1,
						Enabled:         true,
					},
				},
			},
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dinD, err := setupDinD(t, ctx)
			if err != nil {
				t.Fatal(err)
			}
			d, err := docker.NewDockerProvider(dinD.client, zerolog.New(zerolog.NewTestWriter(t)))
			if err != nil {
				t.Fatal(err)
			}

			msgs, errs := d.Events(ctx)

			err = tt.args.do(dinD, d)
			if err != nil {
				t.Fatal(err)
			}

			for i, event := range tt.wantEvents {
				select {
				case msg := <-msgs:
					t.Logf("Event #%d: %v", i, msg)
					assert.Equal(t, event, msg)
				case err := <-errs:
					t.Fatal(err)
				}
			}
		})
	}
}
