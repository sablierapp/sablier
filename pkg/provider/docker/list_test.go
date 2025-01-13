package docker_test

import (
	"context"
	"fmt"
	"github.com/rs/zerolog"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/provider/docker"
	"github.com/sablierapp/sablier/pkg/sablier"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestDockerProvider_List(t *testing.T) {
	ctx := context.Background()
	type args struct {
		do func(dind *dindContainer) error
	}
	tests := []struct {
		name    string
		args    args
		want    []sablier.InstanceConfig
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "no container",
			args: args{
				do: func(dind *dindContainer) error {
					return nil
				},
			},
			want:    []sablier.InstanceConfig{},
			wantErr: assert.NoError,
		},
		{
			name: "1 registered containers with default group",
			args: args{
				do: func(dind *dindContainer) error {
					_, err := dind.CreateMimic(ctx, MimicOptions{
						Name:       "registered",
						Registered: true,
					})
					return err
				},
			},
			want: []sablier.InstanceConfig{
				{
					Name:            "registered",
					Group:           "registered",
					DesiredReplicas: 1,
				},
			},
			wantErr: assert.NoError,
		},

		{
			name: "1 registered container with custom group",
			args: args{
				do: func(dind *dindContainer) error {
					_, err := dind.CreateMimic(ctx, MimicOptions{
						Name:         "registered",
						Registered:   true,
						SablierGroup: "mygroup",
					})
					return err
				},
			},
			want: []sablier.InstanceConfig{
				{
					Name:            "registered",
					Group:           "mygroup",
					DesiredReplicas: 1,
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
			err = tt.args.do(dinD)
			if err != nil {
				t.Fatal(err)
			}
			got, err := d.List(ctx, provider.ListOptions{All: true})
			if !tt.wantErr(t, err, fmt.Sprintf("List")) {
				return
			}
			assert.Equal(t, tt.want, got, "List")
		})
	}
}
