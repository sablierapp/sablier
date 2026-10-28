package sablier

// This file is the single source of truth for the "sablier.*" instance labels.
// Each constant's doc comment documents the label: the first paragraph is the
// description, and the trailing lowercase "key: value" lines carry structured metadata
// (type, default, example, required, since, feature, providers). cmd/labelsgen parses
// these comments to generate the Label reference page, and a test fails if any
// label read in the code is not declared here - so the code, the constants and
// the documentation can never drift apart.
//
//go:generate go run ../../cmd/labelsgen -src labels.go -out ../../docs/content/reference/labels.md

const (
	// LabelEnable opts the instance into Sablier management. Any value other
	// than `true` is ignored.
	//
	// type: boolean
	// required: true
	// example: "true"
	// providers: Kubernetes must set this as a **label** (discovery uses a label
	// selector), never an annotation. Proxmox LXC uses the `sablier` tag instead.
	// Nomad reads the job or task group `meta` block.
	// since: v1.4.0
	LabelEnable = "sablier.enable"

	// LabelGroup assigns the instance to one or more named groups. A session for
	// any of its groups starts the instance.
	//
	// type: comma-separated list
	// default: "default"
	// example: "team-a,team-b"
	// feature: /features/groups/
	// providers: Kubernetes must set a multi-value list as an **annotation**.
	// Proxmox LXC uses one `sablier-group-<name>` tag per group. Nomad reads
	// the job or task group `meta` block. An ECS tag value cannot contain a
	// comma, so ECS separates the groups with a space.
	// since: v1.4.0
	LabelGroup = "sablier.group"

	// LabelReadyAfter is the minimum settling delay after the instance first
	// reports ready before Sablier treats it as ready.
	//
	// type: Go duration
	// example: "30s"
	// feature: /features/ready-after/
	// since: v1.13.0
	LabelReadyAfter = "sablier.ready-after"

	// LabelReadyOnStart treats the instance as ready as soon as the start is
	// dispatched, skipping the health check.
	//
	// type: boolean
	// example: "true"
	// feature: /features/ready-on-start/
	// since: v1.15.0
	LabelReadyOnStart = "sablier.ready-on-start"

	// LabelRunningHours is a daily keep-warm window in local time. Overnight
	// windows like `22:00-06:00` span midnight.
	//
	// type: `HH:MM-HH:MM`
	// example: "09:00-18:00"
	// feature: /features/running-hours/
	// providers: Kubernetes must set the colon value as an **annotation**.
	// since: v1.13.0
	LabelRunningHours = "sablier.running-hours"

	// LabelRunningDays restricts the `sablier.running-hours` window to specific
	// weekdays.
	//
	// type: comma-separated weekdays
	// default: every day
	// example: "Mon,Tue,Wed,Thu,Fri"
	// feature: /features/running-hours/
	// providers: Kubernetes must set the comma value as an **annotation**.
	// ECS separates the days with a space.
	// since: v1.15.0
	LabelRunningDays = "sablier.running-days"

	// LabelAntiAffinity lists the group names this instance backs off from; it is
	// forced idle while any listed group has an active session.
	//
	// type: comma-separated list
	// example: "streaming"
	// feature: /features/anti-affinity/
	// providers: Kubernetes must set a multi-value list as an **annotation**.
	// ECS separates the groups with a space. Not supported on Proxmox LXC.
	// since: v1.15.0
	LabelAntiAffinity = "sablier.anti-affinity"

	// LabelIdleReplicas is the replica count when idle. `0` stops the workload;
	// `1` or more keeps it running with optional resource throttling.
	//
	// type: integer
	// default: `0` (stop)
	// example: "1"
	// feature: /features/scale-mode/
	// providers: Not supported on Proxmox LXC.
	// since: v1.13.0
	LabelIdleReplicas = "sablier.idle.replicas"

	// LabelIdleCPU is the CPU limit applied when the session expires. Requires
	// `sablier.idle.replicas >= 1`.
	//
	// type: decimal cores / Kubernetes quantity
	// example: "0.1"
	// feature: /features/cpu-limits/
	// providers: Kubernetes uses a resource quantity (e.g. `100m`). Not supported
	// on Proxmox LXC, Nomad or ECS.
	// since: v1.13.0
	LabelIdleCPU = "sablier.idle.cpu"

	// LabelIdleMemory is the memory limit applied when the session expires.
	// Requires `sablier.idle.replicas >= 1`.
	//
	// type: Docker units / Kubernetes quantity
	// example: "128m"
	// feature: /features/memory-limits/
	// providers: Kubernetes uses a resource quantity (e.g. `128Mi`). Not
	// supported on Proxmox LXC, Nomad or ECS.
	// since: v1.13.0
	LabelIdleMemory = "sablier.idle.memory"

	// LabelActiveReplicas is the replica count restored when a new session is
	// requested.
	//
	// type: integer
	// default: `1`
	// example: "2"
	// feature: /features/scale-mode/
	// providers: Not supported on Proxmox LXC.
	// since: v1.13.0
	LabelActiveReplicas = "sablier.active.replicas"

	// LabelActiveCPU is the CPU limit restored when a new session is requested.
	//
	// type: decimal cores / Kubernetes quantity
	// example: "2.0"
	// feature: /features/cpu-limits/
	// providers: Kubernetes uses a resource quantity (e.g. `2000m`). Not
	// supported on Proxmox LXC, Nomad or ECS.
	// since: v1.13.0
	LabelActiveCPU = "sablier.active.cpu"

	// LabelActiveMemory is the memory limit restored when a new session is
	// requested.
	//
	// type: Docker units / Kubernetes quantity
	// example: "512m"
	// feature: /features/memory-limits/
	// providers: Kubernetes uses a resource quantity (e.g. `512Mi`). Not
	// supported on Proxmox LXC, Nomad or ECS.
	// since: v1.13.0
	LabelActiveMemory = "sablier.active.memory"

	// LabelIdleBlkioWeight is the relative block I/O scheduling weight applied
	// when idle.
	//
	// type: integer `10`-`1000`
	// example: "100"
	// feature: /features/disk-io-limits/
	// providers: Docker and systemd.
	// since: v1.15.0
	LabelIdleBlkioWeight = "sablier.idle.blkio-weight"

	// LabelActiveBlkioWeight is the relative block I/O scheduling weight restored
	// when active.
	//
	// type: integer `10`-`1000`
	// example: "500"
	// feature: /features/disk-io-limits/
	// providers: Docker and systemd.
	// since: v1.15.0
	LabelActiveBlkioWeight = "sablier.active.blkio-weight"

	// LabelIdleBlkioWeightDevice is the per-device I/O scheduling weight applied
	// when idle.
	//
	// type: `path:weight` list
	// example: "/dev/sda:100"
	// feature: /features/disk-io-limits/
	// providers: Docker (daemon API >= 1.55) and systemd.
	// since: v1.15.0
	LabelIdleBlkioWeightDevice = "sablier.idle.blkio-weight-device"

	// LabelActiveBlkioWeightDevice is the per-device I/O scheduling weight
	// restored when active.
	//
	// type: `path:weight` list
	// example: "/dev/sda:500"
	// feature: /features/disk-io-limits/
	// providers: Docker (daemon API >= 1.55) and systemd.
	// since: v1.15.0
	LabelActiveBlkioWeightDevice = "sablier.active.blkio-weight-device"

	// LabelIdleBlkioReadBps is the per-device read throughput limit applied when
	// idle (Docker byte units, e.g. `10m` = 10 MiB/s).
	//
	// type: `path:rate` list
	// example: "/dev/sda:10m"
	// feature: /features/disk-io-limits/
	// providers: Docker (daemon API >= 1.55) and systemd.
	// since: v1.15.0
	LabelIdleBlkioReadBps = "sablier.idle.blkio-device-read-bps"

	// LabelActiveBlkioReadBps is the per-device read throughput limit restored
	// when active.
	//
	// type: `path:rate` list
	// example: "/dev/sda:100m"
	// feature: /features/disk-io-limits/
	// providers: Docker (daemon API >= 1.55) and systemd.
	// since: v1.15.0
	LabelActiveBlkioReadBps = "sablier.active.blkio-device-read-bps"

	// LabelIdleBlkioWriteBps is the per-device write throughput limit applied
	// when idle.
	//
	// type: `path:rate` list
	// example: "/dev/sda:10m"
	// feature: /features/disk-io-limits/
	// providers: Docker (daemon API >= 1.55) and systemd.
	// since: v1.15.0
	LabelIdleBlkioWriteBps = "sablier.idle.blkio-device-write-bps"

	// LabelActiveBlkioWriteBps is the per-device write throughput limit restored
	// when active.
	//
	// type: `path:rate` list
	// example: "/dev/sda:100m"
	// feature: /features/disk-io-limits/
	// providers: Docker (daemon API >= 1.55) and systemd.
	// since: v1.15.0
	LabelActiveBlkioWriteBps = "sablier.active.blkio-device-write-bps"

	// LabelIdleBlkioReadIOps is the per-device read IOPS limit applied when idle.
	//
	// type: `path:iops` list
	// example: "/dev/sda:100"
	// feature: /features/disk-io-limits/
	// providers: Docker (daemon API >= 1.55) and systemd.
	// since: v1.15.0
	LabelIdleBlkioReadIOps = "sablier.idle.blkio-device-read-iops"

	// LabelActiveBlkioReadIOps is the per-device read IOPS limit restored when
	// active.
	//
	// type: `path:iops` list
	// example: "/dev/sda:1000"
	// feature: /features/disk-io-limits/
	// providers: Docker (daemon API >= 1.55) and systemd.
	// since: v1.15.0
	LabelActiveBlkioReadIOps = "sablier.active.blkio-device-read-iops"

	// LabelIdleBlkioWriteIOps is the per-device write IOPS limit applied when
	// idle.
	//
	// type: `path:iops` list
	// example: "/dev/sda:100"
	// feature: /features/disk-io-limits/
	// providers: Docker (daemon API >= 1.55) and systemd.
	// since: v1.15.0
	LabelIdleBlkioWriteIOps = "sablier.idle.blkio-device-write-iops"

	// LabelActiveBlkioWriteIOps is the per-device write IOPS limit restored when
	// active.
	//
	// type: `path:iops` list
	// example: "/dev/sda:1000"
	// feature: /features/disk-io-limits/
	// providers: Docker (daemon API >= 1.55) and systemd.
	// since: v1.15.0
	LabelActiveBlkioWriteIOps = "sablier.active.blkio-device-write-iops"

	// LabelActivityEnable opts the instance into eBPF activity tracking:
	// traffic on its ports opens or extends the sessions of its groups, like a
	// proxy request. Without a `sablier.group` label, it extends only the
	// instance's own session. Requires `sablier.enable` and
	// `--activity.enabled`.
	//
	// type: boolean
	// default: `false`
	// example: "true"
	// feature: /features/activity/
	// providers: Docker only.
	// since: NEXT_RELEASE
	LabelActivityEnable = "sablier.activity.enable"

	// LabelActivityPorts lists the ports watched for traffic. The protocol is
	// tcp, the default, or udp.
	//
	// type: comma-separated `port[/proto]` list
	// default: the container's exposed TCP and UDP ports
	// example: "32400, 1900/udp"
	// feature: /features/activity/
	// providers: Docker only.
	// since: NEXT_RELEASE
	LabelActivityPorts = "sablier.activity.ports"

	// LabelActivitySessionDuration is the session duration granted by activity.
	// It must be longer than `--activity.interval`, or the instance isn't
	// watched.
	//
	// type: duration
	// default: `--sessions.default-duration`
	// example: "30m"
	// feature: /features/activity/
	// providers: Docker only.
	// since: NEXT_RELEASE
	LabelActivitySessionDuration = "sablier.activity.session-duration"

	// LabelActivityMinBytes is the number of bytes the instance must exchange
	// on its watched ports in one `--activity.interval`, both directions
	// together, to count as active.
	//
	// type: Docker units
	// default: `64k`
	// example: "64k"
	// feature: /features/activity/
	// providers: Docker only.
	// since: NEXT_RELEASE
	LabelActivityMinBytes = "sablier.activity.min-bytes"

	// LabelActivityIgnoreSource marks a container whose traffic never counts as
	// activity, for any watched instance. Set it on the source container,
	// managed by Sablier or not: Sablier ignores that container's current IPs.
	// A container with host networking has no IP of its own: ignore its host's
	// address with `--activity.ignore-cidrs` instead.
	//
	// type: boolean
	// default: `false`
	// example: "true"
	// feature: /features/activity/
	// providers: Docker only.
	// since: NEXT_RELEASE
	LabelActivityIgnoreSource = "sablier.activity.ignore-source"
)
