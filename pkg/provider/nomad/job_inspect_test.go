package nomad_test

import (
	"context"
	"testing"

	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider/nomad"
	"github.com/sablierapp/sablier/pkg/sablier"
	"gotest.tools/v3/assert"
)

func TestNomadProvider_InstanceInspect(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	ctx := context.Background()

	type args struct {
		do func(nc *nomadContainer) (string, error)
	}
	tests := []struct {
		name string
		args args
		want sablier.InstanceInfo
		err  error
	}{
		{
			name: "inspect job with running allocation",
			args: args{
				do: func(nc *nomadContainer) (string, error) {
					job, err := nc.CreateMimicJob(ctx, MimicJobOptions{
						Count: 1,
					})
					if err != nil {
						return "", err
					}

					// Wait for allocation to be running
					err = WaitForJobAllocations(ctx, nc.client, *job.ID, *job.TaskGroups[0].Name, 1)
					if err != nil {
						return "", err
					}

					return formatJobName(*job.ID, *job.TaskGroups[0].Name), nil
				},
			},
			want: sablier.InstanceInfo{
				CurrentReplicas: 1,
				DesiredReplicas: 1,
				Status:          sablier.InstanceStatusReady,
			},
			err: nil,
		},
		{
			name: "inspect job with 0 allocations",
			args: args{
				do: func(nc *nomadContainer) (string, error) {
					job, err := nc.CreateMimicJob(ctx, MimicJobOptions{
						Count: 0,
					})
					if err != nil {
						return "", err
					}

					return formatJobName(*job.ID, *job.TaskGroups[0].Name), nil
				},
			},
			want: sablier.InstanceInfo{
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          sablier.InstanceStatusNotReady,
			},
			err: nil,
		},
	}

	nc := setupNomad(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p, err := nomad.New(ctx, nc.client, "default", slogt.New(t))
			assert.NilError(t, err)

			name, err := tt.args.do(nc)
			assert.NilError(t, err)

			info, err := p.InstanceInspect(ctx, name)
			if tt.err != nil {
				assert.Error(t, err, tt.err.Error())
			} else {
				assert.NilError(t, err)
				assert.Equal(t, name, info.Name)
				assert.Equal(t, tt.want.CurrentReplicas, info.CurrentReplicas)
				assert.Equal(t, tt.want.DesiredReplicas, info.DesiredReplicas)
				assert.Equal(t, tt.want.Status, info.Status)
			}
		})
	}
}
