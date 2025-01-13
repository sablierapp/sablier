package docker_test

import (
	"context"
	"fmt"
	"github.com/docker/docker/api/types/container"
	"github.com/rs/zerolog"
	"github.com/sablierapp/sablier/pkg/provider/docker"
	"github.com/sablierapp/sablier/pkg/sablier"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestDockerProvider_Info(t *testing.T) {
	ctx := context.Background()
	type args struct {
		do   func(dind *dindContainer) error
		name string
	}
	tests := []struct {
		name    string
		args    args
		want    sablier.InstanceInfo
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "container is created",
			args: args{
				do: func(dind *dindContainer) error {
					_, err := dind.CreateMimic(ctx, MimicOptions{
						Name: "test-info-created",
					})
					return err
				},
				name: "test-info-created",
			},
			want: sablier.InstanceInfo{
				Name:            "test-info-created",
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          sablier.InstanceDown,
				StartedAt:       time.Time{},
			},
			wantErr: assert.NoError,
		},
		{
			name: "container is paused",
			args: args{
				do: func(dind *dindContainer) error {
					mimic, err := dind.CreateMimic(ctx, MimicOptions{
						Name: "test-info-paused",
					})
					if err != nil {
						return err
					}
					err = dind.client.ContainerStart(ctx, mimic.ID, container.StartOptions{})
					if err != nil {
						return err
					}
					return dind.client.ContainerPause(ctx, mimic.ID)
				},
				name: "test-info-paused",
			},
			want: sablier.InstanceInfo{
				Name:            "test-info-paused",
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          sablier.InstanceDown,
				StartedAt:       time.Time{},
			},
			wantErr: assert.NoError,
		},
		{
			name: "container is exited",
			args: args{
				do: func(dind *dindContainer) error {
					mimic, err := dind.CreateMimic(ctx, MimicOptions{
						Name: "test-info-exited",
					})
					if err != nil {
						return err
					}
					err = dind.client.ContainerStart(ctx, mimic.ID, container.StartOptions{})
					if err != nil {
						return err
					}
					return dind.client.ContainerStop(ctx, mimic.ID, container.StopOptions{})
				},
				name: "test-info-exited",
			},
			want: sablier.InstanceInfo{
				Name:            "test-info-exited",
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          sablier.InstanceDown,
				StartedAt:       time.Time{},
			},
			wantErr: assert.NoError,
		},
		{
			name: "container is running (no healthcheck)",
			args: args{
				do: func(dind *dindContainer) error {
					mimic, err := dind.CreateMimic(ctx, MimicOptions{
						Name: "test-info-running-no-healthcheck",
					})
					if err != nil {
						return err
					}
					return dind.client.ContainerStart(ctx, mimic.ID, container.StartOptions{})
				},
				name: "test-info-running-no-healthcheck",
			},
			want: sablier.InstanceInfo{
				Name:            "test-info-running-no-healthcheck",
				CurrentReplicas: 1,
				DesiredReplicas: 1,
				Status:          sablier.InstanceReady,
				StartedAt:       time.Now(),
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
			err = tt.args.do(dinD)
			if err != nil {
				t.Fatal(err)
			}
			got, err := d.Info(ctx, tt.args.name)
			if !tt.wantErr(t, err, fmt.Sprintf("Info(ctx, %v)", tt.args.name)) {
				return
			}
			assert.NotNil(t, got.StartedAt)
			// assert.NotEqual(t, time.Time{}, got.StartedAt) // When instance is not started, this is not set
			got.StartedAt = tt.want.StartedAt // Cannot assert equal on that field otherwise
			assert.Equalf(t, tt.want, got, "Info(ctx, %v)", tt.args.name)
		})
	}
}
