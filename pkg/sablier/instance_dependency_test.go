package sablier

import "testing"

func TestDependencyConditionSatisfied(t *testing.T) {
	tests := []struct {
		name      string
		status    InstanceStatus
		condition string
		want      bool
	}{
		// service_started: accepts Running-equivalent statuses.
		{name: "started/starting", status: InstanceStatusStarting, condition: "service_started", want: true},
		{name: "started/ready", status: InstanceStatusReady, condition: "service_started", want: true},
		{name: "started/stopped", status: InstanceStatusStopped, condition: "service_started", want: false},
		{name: "started/error", status: InstanceStatusError, condition: "service_started", want: false},

		// service_healthy: requires Ready (healthy container).
		{name: "healthy/ready", status: InstanceStatusReady, condition: "service_healthy", want: true},
		{name: "healthy/starting", status: InstanceStatusStarting, condition: "service_healthy", want: false},
		{name: "healthy/stopped", status: InstanceStatusStopped, condition: "service_healthy", want: false},

		// service_completed_successfully: requires Ready (exited-0 with HonorRestartPolicy).
		{name: "completed/ready", status: InstanceStatusReady, condition: "service_completed_successfully", want: true},
		{name: "completed/starting", status: InstanceStatusStarting, condition: "service_completed_successfully", want: false},
		{name: "completed/stopped", status: InstanceStatusStopped, condition: "service_completed_successfully", want: false},

		// Unknown condition: same as service_started.
		{name: "unknown/starting", status: InstanceStatusStarting, condition: "unknown_condition", want: true},
		{name: "unknown/ready", status: InstanceStatusReady, condition: "unknown_condition", want: true},
		{name: "unknown/stopped", status: InstanceStatusStopped, condition: "unknown_condition", want: false},

		// Empty condition: same as service_started.
		{name: "empty/starting", status: InstanceStatusStarting, condition: "", want: true},
		{name: "empty/stopped", status: InstanceStatusStopped, condition: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := dependencyConditionSatisfied(tt.status, tt.condition); got != tt.want {
				t.Errorf("dependencyConditionSatisfied(%v, %q) = %v, want %v",
					tt.status, tt.condition, got, tt.want)
			}
		})
	}
}
