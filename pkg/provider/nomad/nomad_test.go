package nomad

import (
	"testing"

	"github.com/hashicorp/nomad/api"
	"gotest.tools/v3/assert"
)

func TestParseName(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    parsedName
		wantErr string
	}{
		{
			name: "job and group",
			in:   "whoami/web",
			want: parsedName{JobID: "whoami", Group: "web"},
		},
		{
			name: "job id containing a slash uses the last separator",
			in:   "cron/dispatch-123/worker",
			want: parsedName{JobID: "cron/dispatch-123", Group: "worker"},
		},
		{
			name:    "missing group",
			in:      "whoami",
			wantErr: `instance name "whoami" must be in the form job/group`,
		},
		{
			name:    "empty group",
			in:      "whoami/",
			wantErr: `instance name "whoami/" must be in the form job/group`,
		},
		{
			name:    "empty job",
			in:      "/web",
			wantErr: `instance name "/web" must be in the form job/group`,
		},
		{
			name:    "empty name",
			in:      "",
			wantErr: `instance name "" must be in the form job/group`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseName(tt.in)
			if tt.wantErr != "" {
				assert.Error(t, err, tt.wantErr)
				return
			}
			assert.NilError(t, err)
			assert.DeepEqual(t, got, tt.want)
		})
	}
}

func TestInstanceName(t *testing.T) {
	assert.Equal(t, instanceName("whoami", "web"), "whoami/web")
	assert.Equal(t, instanceName("cron/dispatch-123", "worker"), "cron/dispatch-123/worker")
}

func TestGroupMeta(t *testing.T) {
	t.Run("task group meta overrides job meta", func(t *testing.T) {
		job := &api.Job{Meta: map[string]string{
			"sablier.enable": "true",
			"sablier.group":  "job-level",
		}}
		tg := &api.TaskGroup{Meta: map[string]string{
			"sablier.group": "group-level",
		}}

		got := groupMeta(job, tg)

		assert.DeepEqual(t, got, map[string]string{
			"sablier.enable": "true",
			"sablier.group":  "group-level",
		})
		// The inputs are left untouched.
		assert.DeepEqual(t, job.Meta, map[string]string{"sablier.enable": "true", "sablier.group": "job-level"})
		assert.DeepEqual(t, tg.Meta, map[string]string{"sablier.group": "group-level"})
	})

	t.Run("nil meta yields an empty map", func(t *testing.T) {
		got := groupMeta(&api.Job{}, &api.TaskGroup{})
		assert.Assert(t, got != nil)
		assert.Equal(t, len(got), 0)
	})
}
