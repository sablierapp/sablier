package docker

// Unit tests for parsing the Docker Compose depends_on label.
// These run without a real Docker daemon.

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseComposeDependsOn(t *testing.T) {
	tests := []struct {
		name  string
		label string
		want  []composeDependency
	}{
		{
			name:  "empty label",
			label: "",
			want:  nil,
		},
		{
			name:  "whitespace only label",
			label: "   ",
			want:  nil,
		},
		{
			name:  "single dependency",
			label: "db:service_healthy:false",
			want: []composeDependency{
				{Service: "db", Condition: "service_healthy"},
			},
		},
		{
			name:  "multiple dependencies",
			label: "db:service_healthy:false,migration:service_completed_successfully:true",
			want: []composeDependency{
				{Service: "db", Condition: "service_healthy"},
				{Service: "migration", Condition: "service_completed_successfully"},
			},
		},
		{
			name:  "dependency without restart flag",
			label: "db:service_started",
			want: []composeDependency{
				{Service: "db", Condition: "service_started"},
			},
		},
		{
			name:  "skips malformed entries",
			label: "db:service_healthy:false,,broken,:service_started:false,ok:service_started:false",
			want: []composeDependency{
				{Service: "db", Condition: "service_healthy"},
				{Service: "ok", Condition: "service_started"},
			},
		},
		{
			name:  "trims whitespace around entries",
			label: " db:service_healthy:false , migration:service_completed_successfully:false ",
			want: []composeDependency{
				{Service: "db", Condition: "service_healthy"},
				{Service: "migration", Condition: "service_completed_successfully"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseComposeDependsOn(tt.label)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("parseComposeDependsOn() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
