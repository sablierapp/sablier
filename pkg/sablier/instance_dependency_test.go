package sablier

import "testing"

func TestDependencyConditionSatisfied(t *testing.T) {
	tests := []struct {
		name      string
		status    InstanceStatus
		condition string
		want      bool
	}{
		// service_started: accepts any started status, including a completed one-shot.
		{name: "started/starting", status: InstanceStatusStarting, condition: "service_started", want: true},
		{name: "started/ready", status: InstanceStatusReady, condition: "service_started", want: true},
		{name: "started/completed", status: InstanceStatusCompleted, condition: "service_started", want: true},
		{name: "started/stopped", status: InstanceStatusStopped, condition: "service_started", want: false},
		{name: "started/error", status: InstanceStatusError, condition: "service_started", want: false},

		// service_healthy: requires Ready (healthy container). A completed one-shot is not healthy.
		{name: "healthy/ready", status: InstanceStatusReady, condition: "service_healthy", want: true},
		{name: "healthy/starting", status: InstanceStatusStarting, condition: "service_healthy", want: false},
		{name: "healthy/completed", status: InstanceStatusCompleted, condition: "service_healthy", want: false},
		{name: "healthy/stopped", status: InstanceStatusStopped, condition: "service_healthy", want: false},

		// service_completed_successfully: requires Completed (exited-0 one-shot). A running Ready service is not completed.
		{name: "completed/completed", status: InstanceStatusCompleted, condition: "service_completed_successfully", want: true},
		{name: "completed/ready", status: InstanceStatusReady, condition: "service_completed_successfully", want: false},
		{name: "completed/starting", status: InstanceStatusStarting, condition: "service_completed_successfully", want: false},
		{name: "completed/stopped", status: InstanceStatusStopped, condition: "service_completed_successfully", want: false},

		// service_running_or_healthy: requires a running (or healthy) service, not a completed one-shot.
		{name: "running_or_healthy/starting", status: InstanceStatusStarting, condition: "service_running_or_healthy", want: true},
		{name: "running_or_healthy/ready", status: InstanceStatusReady, condition: "service_running_or_healthy", want: true},
		{name: "running_or_healthy/completed", status: InstanceStatusCompleted, condition: "service_running_or_healthy", want: false},
		{name: "running_or_healthy/stopped", status: InstanceStatusStopped, condition: "service_running_or_healthy", want: false},

		// Unknown condition: same as service_started.
		{name: "unknown/starting", status: InstanceStatusStarting, condition: "unknown_condition", want: true},
		{name: "unknown/ready", status: InstanceStatusReady, condition: "unknown_condition", want: true},
		{name: "unknown/completed", status: InstanceStatusCompleted, condition: "unknown_condition", want: true},
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
