package theme

import (
	"time"

	"github.com/sablierapp/sablier/pkg/sablier"
)

// Theme represents an available theme
type Theme struct {
	Name     string
	Embedded bool
}

// Instance holds the current state about an instance
type Instance struct {
	Name            string                          `jsonschema:"description=Name of the instance as registered with the provider,example=nginx"`
	Status          string                          `jsonschema:"description=Current status of the instance (starting|ready|not-ready|error|not-found),example=starting"`
	Error           error                           `json:",omitempty" jsonschema:"description=Error encountered while resolving the instance state if any"`
	CurrentReplicas int32                           `jsonschema:"description=Current number of running replicas,example=0"`
	DesiredReplicas int32                           `jsonschema:"description=Target number of replicas to reach before the session is considered ready,example=1"`
	Provider        string                          `jsonschema:"description=Provider type (docker|swarm|kubernetes|podman),example=docker"`
	Docker          *sablier.DockerContainerInfo    `json:",omitempty" jsonschema:"description=Docker-specific container metadata (only set when Provider is docker). Fields are accessible in templates as .Docker.ID .Docker.Image .Docker.Labels"`
	Swarm           *sablier.SwarmServiceInfo       `json:",omitempty" jsonschema:"description=Docker Swarm-specific service metadata (only set when Provider is swarm). Fields are accessible in templates as .Swarm.ID .Swarm.Image .Swarm.Labels"`
	Kubernetes      *sablier.KubernetesWorkloadInfo `json:",omitempty" jsonschema:"description=Kubernetes-specific workload metadata (only set when Provider is kubernetes). Fields are accessible in templates as .Kubernetes.Namespace .Kubernetes.Kind .Kubernetes.Image .Kubernetes.Labels"`
	Podman          *sablier.PodmanContainerInfo    `json:",omitempty" jsonschema:"description=Podman-specific container metadata (only set when Provider is podman). Fields are accessible in templates as .Podman.ID .Podman.Image .Podman.Labels"`
}

// Options holds the customizable input to template
type Options struct {
	DisplayName      string
	ShowDetails      bool
	InstanceStates   []Instance
	SessionDuration  time.Duration
	RefreshFrequency time.Duration
}

// TemplateData is the data passed to a theme template when rendering a loading page.
// All fields listed here are available as template variables (e.g. {{.DisplayName}}).
type TemplateData struct {
	DisplayName      string     `jsonschema:"description=Optional title shown on the loading page. Empty when no display name was configured.,example=My Application"`
	InstanceStates   []Instance `jsonschema:"description=Current state of each requested instance. Only populated when the request includes show_details=true."`
	SessionDuration  string     `jsonschema:"description=Human-readable remaining session duration (e.g. '1 hour 30 minutes').,example=1 hour 30 minutes"`
	RefreshFrequency string     `jsonschema:"description=Page auto-refresh interval in whole seconds as a string (e.g. '30').,example=5"`
	Version          string     `jsonschema:"description=Current Sablier server version string (e.g. '1.8.0').,example=1.8.0"`
}
