# Theme Template Variables

This page is generated automatically from the Sablier binary. Do not edit by hand.

The following variables are available inside every Sablier HTML theme template.

| Variable | Type | Description |
|----------|------|-------------|
| `.display_name` | `string` | A human-readable title displayed in the loading page header. |
| `.show_details` | `bool` | Whether to show per-instance status rows in the loading page. |
| `.instance_states` | `[]Instance` | List of instances and their current status. |
| `.session_duration` | `duration` | How long the session remains alive after the last request. |
| `.refresh_frequency` | `duration` | How often the loading page polls for updated instance status. |

## Instance Fields

Each entry in `.instance_states` exposes the following fields:

| Variable | Type | Description |
|----------|------|-------------|
| `.name` | `string` | The instance name. |
| `.status` | `string` | Current status: `ready`, `not-ready`, `starting`, or `error`. |
| `.error` | `error` | Error message when status is `error`. |
| `.current_replicas` | `int32` | Number of currently running replicas. |
| `.desired_replicas` | `int32` | Number of desired replicas. |
| `.provider` | `string` | Provider type (e.g. `docker`, `kubernetes`). |
| `.docker` | `*DockerContainerInfo` | Docker-specific metadata (nil for other providers). |
| `.swarm` | `*SwarmServiceInfo` | Docker Swarm-specific metadata (nil for other providers). |
| `.kubernetes` | `*KubernetesWorkloadInfo` | Kubernetes-specific metadata (nil for other providers). |
| `.podman` | `*PodmanContainerInfo` | Podman-specific metadata (nil for other providers). |
