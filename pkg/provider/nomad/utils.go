package nomad

import (
	"fmt"
	"strings"
)

// parseJobName extracts job ID and task group name from the instance name
// Expected format: "jobID/taskGroupName" or just "jobID" (uses default group)
func parseJobName(name string) (string, string, error) {
	parts := strings.Split(name, "/")

	if len(parts) == 1 {
		// If only job ID provided, use default group name
		return parts[0], parts[0], nil
	}

	if len(parts) == 2 {
		if parts[0] == "" || parts[1] == "" {
			return "", "", fmt.Errorf("invalid job name format: %s (expected 'jobID/taskGroupName')", name)
		}
		return parts[0], parts[1], nil
	}

	return "", "", fmt.Errorf("invalid job name format: %s (expected 'jobID/taskGroupName' or 'jobID')", name)
}

// formatJobName creates the instance name from job ID and task group name
func formatJobName(jobID string, taskGroupName string) string {
	return fmt.Sprintf("%s/%s", jobID, taskGroupName)
}
