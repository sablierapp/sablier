package nomad_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider/nomad"
	"gotest.tools/v3/assert"
)

func TestNomadProvider_InstanceStop(t *testing.T) {
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
		err  error
	}{
		{
			name: "stop job with running allocations",
			args: args{
				do: func(nc *nomadContainer) (string, error) {
					job, err := nc.CreateMimicJob(ctx, MimicJobOptions{
						Count: 1,
						Meta: map[string]string{
							"sablier.enable": "true",
						},
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
			err: nil,
		},
		{
			name: "stop job already at 0",
			args: args{
				do: func(nc *nomadContainer) (string, error) {
					job, err := nc.CreateMimicJob(ctx, MimicJobOptions{
						Count: 0,
						Meta: map[string]string{
							"sablier.enable": "true",
						},
					})
					if err != nil {
						return "", err
					}
					return formatJobName(*job.ID, *job.TaskGroups[0].Name), nil
				},
			},
			err: nil,
		},
		{
			name: "stop non-existent job",
			args: args{
				do: func(nc *nomadContainer) (string, error) {
					return "non-existent/taskgroup", nil
				},
			},
			err: fmt.Errorf("job not found"),
		},
	}

	nc := setupNomad(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := nomad.New(ctx, nc.client, "default", slogt.New(t))
			assert.NilError(t, err)

			name, err := tt.args.do(nc)
			assert.NilError(t, err)

			err = p.InstanceStop(ctx, name)
			if tt.err != nil {
				assert.ErrorContains(t, err, "job not found")
			} else {
				assert.NilError(t, err)

				// Verify the job was scaled to 0
				info, err := p.InstanceInspect(ctx, name)
				assert.NilError(t, err)
				assert.Equal(t, int32(0), info.DesiredReplicas)
			}
		})
	}
}
