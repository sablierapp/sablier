package sablier

import (
	"log/slog"
	"strconv"
	"strings"
	"time"
)

type InstanceStatus string

const (
	InstanceStatusStopped  InstanceStatus = "stopped"
	InstanceStatusStarting InstanceStatus = "starting"
	InstanceStatusReady    InstanceStatus = "ready"
	InstanceStatusError    InstanceStatus = "error"
	// InstanceStatusCompleted is the terminal state of a one-shot / init
	// workload that ran and exited successfully (exit code 0) and is not
	// expected to run again. It is distinct from Ready (running and serving
	// traffic): a completed instance is not running. It satisfies a
	// service_completed_successfully dependency but never a service_healthy one.
	InstanceStatusCompleted InstanceStatus = "completed"
	// InstanceStatusNotReady marks an instance that Sablier is deliberately not
	// starting yet — currently only when it is held back by an active
	// anti-affinity antagonist group. It carries a Message explaining why, is
	// never persisted or seen by providers, and is treated as not ready.
	InstanceStatusNotReady InstanceStatus = "not-ready"
)

// ProviderType identifies the infrastructure provider that manages an instance.
type ProviderType = string

const (
	ProviderDocker     ProviderType = "docker"
	ProviderSwarm      ProviderType = "swarm"
	ProviderKubernetes ProviderType = "kubernetes"
	ProviderPodman     ProviderType = "podman"
)

type InstanceInfo struct {
	Name            string                  `json:"name"`
	CurrentReplicas int32                   `json:"currentReplicas"`
	DesiredReplicas int32                   `json:"desiredReplicas"`
	Status          InstanceStatus          `json:"status"`
	Groups          []string                `json:"groups,omitempty"`
	Enabled         string                  `json:"enabled,omitempty"`
	Message         string                  `json:"message,omitempty"`
	Provider        ProviderType            `json:"provider,omitempty"`
	Docker          *DockerContainerInfo    `json:"docker,omitempty"`
	Swarm           *SwarmServiceInfo       `json:"swarm,omitempty"`
	Kubernetes      *KubernetesWorkloadInfo `json:"kubernetes,omitempty"`
	Podman          *PodmanContainerInfo    `json:"podman,omitempty"`

	// ReadyAfter is the minimum duration to wait after the instance first
	// reports ready before Sablier considers it truly ready. Set via the
	// sablier.ready-after label (e.g. "30s"). Zero means no extra wait.
	ReadyAfter time.Duration `json:"readyAfter,omitempty"`
	// ReadyAt records when the instance first transitioned to InstanceStatusReady.
	// It is set internally by Sablier and is never populated by a provider.
	ReadyAt *time.Time `json:"readyAt,omitempty"`

	// RunningHours is a daily keep-warm window in local time, parsed from
	// the sablier.running-hours label (format: HH:MM-HH:MM).
	RunningHours string `json:"runningHours,omitempty"`

	// ReadyOnStart indicates the instance should be considered ready as soon as
	// the start is dispatched, without waiting for health.
	// Controlled by the sablier.ready-on-start label.
	ReadyOnStart bool `json:"readyOnStart,omitempty"`

	// AntiAffinity lists the groups this instance backs off from. Whenever any
	// listed group has an active session, Sablier forces this instance to its
	// idle state (stopped, or idle resources in scale mode) and restores it once
	// none of the listed groups are active anymore. Parsed from the
	// sablier.anti-affinity label (comma-separated group names).
	AntiAffinity []string `json:"antiAffinity,omitempty"`

	// ScaleConfig configures resource-based scale mode for this instance.
	// When present, Sablier throttles CPU/memory instead of stopping the container.
	ScaleConfig *ScaleConfig `json:"scaleConfig,omitempty"`
}

// BlkioWeightDevice holds a per-device I/O scheduling weight override.
type BlkioWeightDevice struct {
	Path   string `json:"path"`
	Weight uint16 `json:"weight"` // valid range: 10–1000
}

// BlkioThrottleDevice holds a per-device I/O rate constraint.
// Rate is the raw label value: human-readable bytes for bps (e.g. "10m", "100k")
// or a plain integer for iops (e.g. "100"). Conversion to the numeric wire format
// is performed by the provider.
type BlkioThrottleDevice struct {
	Path string `json:"path"`
	Rate string `json:"rate"`
}

// ScaleConfig defines the idle and active resource profiles used in scale mode.
// In scale mode, the container keeps running but its resources are adjusted:
// idle resources are applied when the session expires, active resources when
// a new session is requested.
type ScaleConfig struct {
	Idle   ResourceProfile `json:"idle"`
	Active ResourceProfile `json:"active"`
}

// ResourceProfile holds the CPU, memory, and I/O limits for a single resource profile.
type ResourceProfile struct {
	// Replicas is the desired replica count for this profile.
	// For idle: 0 (default) stops the workload; ≥ 1 keeps it running with
	// throttled resources (resource scaling mode).
	// For active: defaults to 1.
	Replicas int32 `json:"replicas,omitempty"`
	// CPU is the CPU limit (e.g. "0.5" for Docker/Swarm, "500m" for Kubernetes).
	CPU string `json:"cpu,omitempty"`
	// Memory is the memory limit (e.g. "128m" for Docker/Swarm, "128Mi" for Kubernetes).
	Memory string `json:"memory,omitempty"`
	// BlkioWeight is the relative block I/O scheduling weight (10–1000).
	// 0 means unset (use the container's current or default weight).
	// Supported on Docker and Podman; ignored on Docker Swarm and Kubernetes.
	BlkioWeight uint16 `json:"blkioWeight,omitempty"`
	// BlkioWeightDevice overrides the I/O scheduling weight per block device.
	BlkioWeightDevice []BlkioWeightDevice `json:"blkioWeightDevice,omitempty"`
	// BlkioDeviceReadBps limits the read throughput per block device (bytes/s).
	// Rate values use human-readable suffixes, e.g. "10m" = 10 MB/s.
	BlkioDeviceReadBps []BlkioThrottleDevice `json:"blkioDeviceReadBps,omitempty"`
	// BlkioDeviceWriteBps limits the write throughput per block device (bytes/s).
	BlkioDeviceWriteBps []BlkioThrottleDevice `json:"blkioDeviceWriteBps,omitempty"`
	// BlkioDeviceReadIOps limits the read IOPS per block device.
	// Rate values are plain integers, e.g. "100".
	BlkioDeviceReadIOps []BlkioThrottleDevice `json:"blkioDeviceReadIOps,omitempty"`
	// BlkioDeviceWriteIOps limits the write IOPS per block device.
	BlkioDeviceWriteIOps []BlkioThrottleDevice `json:"blkioDeviceWriteIOps,omitempty"`
}

// HasResources reports whether any resource constraint (CPU, memory, or any
// blkio field) is set on this profile. It does not consider Replicas.
func (r ResourceProfile) HasResources() bool {
	return r.CPU != "" || r.Memory != "" || r.BlkioWeight != 0 ||
		r.HasBlkioDeviceLimits()
}

// HasBlkioDeviceLimits reports whether any per-device blkio constraint is set on
// this profile. These fields require a Docker daemon API version >= 1.55 to be
// honored on a running container (see moby/moby#52650); the global BlkioWeight
// field is not affected.
func (r ResourceProfile) HasBlkioDeviceLimits() bool {
	return len(r.BlkioWeightDevice) > 0 ||
		len(r.BlkioDeviceReadBps) > 0 ||
		len(r.BlkioDeviceWriteBps) > 0 ||
		len(r.BlkioDeviceReadIOps) > 0 ||
		len(r.BlkioDeviceWriteIOps) > 0
}

type InstanceConfiguration struct {
	Name    string
	Groups  []string
	Enabled string
}

func (instance InstanceInfo) IsReady() bool {
	if instance.ReadyOnStart {
		return true
	}
	if instance.Status != InstanceStatusReady {
		return false
	}
	if instance.ReadyAfter == 0 || instance.ReadyAt == nil {
		return true
	}
	return time.Since(*instance.ReadyAt) >= instance.ReadyAfter
}

// ScaleConfigFromLabels extracts a ScaleConfig from the given label map.
// It always returns a value. When none of the scale labels
// (sablier.idle.{cpu,memory,replicas,blkio-weight}, sablier.active.{cpu,memory,replicas,blkio-weight})
// are present it returns a zero-value struct with defaults:
//   - Idle.Replicas = 0 (workload is stopped when idle)
//   - Active.Replicas = 1 (workload runs with a single replica when active)
//
// Callers detect whether scale mode is active via field values:
//   - Stop path:  sc.Idle.Replicas >= 1
//   - Start path: sc.Idle.Replicas >= 1 || sc.Active.Replicas > 1 || sc.Active.CPU != "" || sc.Active.Memory != "" || sc.Active.BlkioWeight != 0
//
// Resource scaling (CPU/memory/blkio throttling instead of stopping) is only applied
// when Idle.Replicas ≥ 1.
func ScaleConfigFromLabels(labels map[string]string) ScaleConfig {
	idleCPU := labels["sablier.idle.cpu"]
	idleMemory := labels["sablier.idle.memory"]
	activeCPU := labels["sablier.active.cpu"]
	activeMemory := labels["sablier.active.memory"]

	idleReplicas := int32(0) // default: stop the workload
	if v, ok := labels["sablier.idle.replicas"]; ok {
		if n, err := strconv.ParseInt(v, 10, 32); err == nil && n >= 0 {
			idleReplicas = int32(n)
		}
	}

	activeReplicas := int32(1) // default: one running replica
	if v, ok := labels["sablier.active.replicas"]; ok {
		if n, err := strconv.ParseInt(v, 10, 32); err == nil && n >= 0 {
			activeReplicas = int32(n)
		}
	}

	var idleBlkioWeight, activeBlkioWeight uint16
	if v, ok := labels["sablier.idle.blkio-weight"]; ok {
		if n, err := strconv.ParseUint(v, 10, 16); err == nil && n >= 10 && n <= 1000 {
			idleBlkioWeight = uint16(n)
		}
	}
	if v, ok := labels["sablier.active.blkio-weight"]; ok {
		if n, err := strconv.ParseUint(v, 10, 16); err == nil && n >= 10 && n <= 1000 {
			activeBlkioWeight = uint16(n)
		}
	}

	var idleWeightDevice []BlkioWeightDevice
	var idleReadBps, idleWriteBps, idleReadIOps, idleWriteIOps []BlkioThrottleDevice
	var activeWeightDevice []BlkioWeightDevice
	var activeReadBps, activeWriteBps, activeReadIOps, activeWriteIOps []BlkioThrottleDevice

	if v := labels["sablier.idle.blkio-weight-device"]; v != "" {
		idleWeightDevice = parseWeightDevices(v)
	}
	if v := labels["sablier.idle.blkio-device-read-bps"]; v != "" {
		idleReadBps = parseThrottleDevices(v)
	}
	if v := labels["sablier.idle.blkio-device-write-bps"]; v != "" {
		idleWriteBps = parseThrottleDevices(v)
	}
	if v := labels["sablier.idle.blkio-device-read-iops"]; v != "" {
		idleReadIOps = parseThrottleDevices(v)
	}
	if v := labels["sablier.idle.blkio-device-write-iops"]; v != "" {
		idleWriteIOps = parseThrottleDevices(v)
	}

	if v := labels["sablier.active.blkio-weight-device"]; v != "" {
		activeWeightDevice = parseWeightDevices(v)
	}
	if v := labels["sablier.active.blkio-device-read-bps"]; v != "" {
		activeReadBps = parseThrottleDevices(v)
	}
	if v := labels["sablier.active.blkio-device-write-bps"]; v != "" {
		activeWriteBps = parseThrottleDevices(v)
	}
	if v := labels["sablier.active.blkio-device-read-iops"]; v != "" {
		activeReadIOps = parseThrottleDevices(v)
	}
	if v := labels["sablier.active.blkio-device-write-iops"]; v != "" {
		activeWriteIOps = parseThrottleDevices(v)
	}

	return ScaleConfig{
		Idle: ResourceProfile{
			Replicas:             idleReplicas,
			CPU:                  idleCPU,
			Memory:               idleMemory,
			BlkioWeight:          idleBlkioWeight,
			BlkioWeightDevice:    idleWeightDevice,
			BlkioDeviceReadBps:   idleReadBps,
			BlkioDeviceWriteBps:  idleWriteBps,
			BlkioDeviceReadIOps:  idleReadIOps,
			BlkioDeviceWriteIOps: idleWriteIOps,
		},
		Active: ResourceProfile{
			Replicas:             activeReplicas,
			CPU:                  activeCPU,
			Memory:               activeMemory,
			BlkioWeight:          activeBlkioWeight,
			BlkioWeightDevice:    activeWeightDevice,
			BlkioDeviceReadBps:   activeReadBps,
			BlkioDeviceWriteBps:  activeWriteBps,
			BlkioDeviceReadIOps:  activeReadIOps,
			BlkioDeviceWriteIOps: activeWriteIOps,
		},
	}
}

// parseWeightDevices parses a comma-separated list of "path:weight" pairs.
// Entries with out-of-range weights (10–1000) or malformed format are skipped.
func parseWeightDevices(s string) []BlkioWeightDevice {
	var out []BlkioWeightDevice
	for _, entry := range strings.Split(s, ",") {
		entry = strings.TrimSpace(entry)
		i := strings.LastIndex(entry, ":")
		if i <= 0 {
			continue
		}
		path, wstr := entry[:i], strings.TrimSpace(entry[i+1:])
		n, err := strconv.ParseUint(wstr, 10, 16)
		if err != nil || n < 10 || n > 1000 {
			continue
		}
		out = append(out, BlkioWeightDevice{Path: path, Weight: uint16(n)})
	}
	return out
}

// parseThrottleDevices parses a comma-separated list of "path:rate" pairs.
// The rate is kept as a raw string; conversion to bytes or IOPS is the
// provider's responsibility. Entries with a missing path or empty rate are skipped.
func parseThrottleDevices(s string) []BlkioThrottleDevice {
	var out []BlkioThrottleDevice
	for _, entry := range strings.Split(s, ",") {
		entry = strings.TrimSpace(entry)
		i := strings.LastIndex(entry, ":")
		if i <= 0 {
			continue
		}
		path, rate := entry[:i], strings.TrimSpace(entry[i+1:])
		if path == "" || rate == "" {
			continue
		}
		out = append(out, BlkioThrottleDevice{Path: path, Rate: rate})
	}
	return out
}

// ParseGroups parses a comma-separated group label value into a deduplicated slice.
// Returns []string{"default"} if the label is empty.
func ParseGroups(label string) []string {
	if label == "" {
		return []string{"default"}
	}
	parts := strings.Split(label, ",")
	seen := make(map[string]bool, len(parts))
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" && !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return []string{"default"}
	}
	return out
}

// ParseAntiAffinity parses a comma-separated anti-affinity label value into a
// deduplicated slice of group names. Unlike ParseGroups it has no "default"
// fallback: an empty or whitespace-only value yields nil (no anti-affinity).
func ParseAntiAffinity(label string) []string {
	parts := strings.Split(label, ",")
	seen := make(map[string]bool, len(parts))
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" && !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// PopulateEnabledAndGroup reads the sablier.enable and sablier.group labels from
// labels and writes the results into info. Centralising this logic avoids
// duplicating the same map lookups in every provider's Inspect implementation.
func PopulateEnabledAndGroup(info *InstanceInfo, labels map[string]string) {
	info.Enabled = labels["sablier.enable"]
	if info.Enabled == "true" {
		info.Groups = ParseGroups(labels["sablier.group"])
	}
	if v := labels["sablier.ready-after"]; v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			info.ReadyAfter = d
		} else {
			slog.Warn("invalid sablier.ready-after label value, ignoring",
				slog.String("instance", info.Name),
				slog.String("value", v),
				slog.Any("error", err),
			)
		}
	}
	if v := labels["sablier.running-hours"]; v != "" {
		if _, err := ParseRunningHours(v); err == nil {
			info.RunningHours = v
		} else {
			slog.Warn("invalid sablier.running-hours label value, ignoring",
				slog.String("instance", info.Name),
				slog.String("value", v),
				slog.Any("error", err),
			)
		}
	}
	if v := labels["sablier.ready-on-start"]; v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			slog.Warn("invalid sablier.ready-on-start label value, ignoring",
				slog.String("instance", info.Name),
				slog.String("value", v),
				slog.Any("error", err),
			)
		} else {
			info.ReadyOnStart = b
		}
	}
	if v := labels["sablier.anti-affinity"]; v != "" {
		info.AntiAffinity = ParseAntiAffinity(v)
	}
	// Only expose ScaleConfig in the response when at least one non-default
	// scale label is present. Detects configuration by checking for values
	// that differ from the zero-value defaults (Idle.Replicas=0, Active.Replicas=1).
	sc := ScaleConfigFromLabels(labels)
	if sc.Idle.Replicas > 0 || sc.Idle.HasResources() ||
		sc.Active.Replicas > 1 || sc.Active.HasResources() {
		info.ScaleConfig = &sc
	}
}
