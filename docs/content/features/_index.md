---
title: Features
weight: 70
---

Sablier's per-instance behavior is driven by labels (or Proxmox tags). Each feature below documents one capability: when to use it, the labels it introduces, examples, and provider-specific notes.

The complete label list lives in the [Configuration reference](/configuration/#instance-labels), and the [compatibility matrix](/compatibility/) shows which features work on which providers.

{{< cards >}}
  {{< card link="/features/scale-mode/" icon="chip" title="Scale mode" subtitle="Throttle CPU, memory and block I/O when idle instead of stopping the workload." >}}
  {{< card link="/features/running-hours/" icon="clock" title="Running hours" subtitle="Keep an instance warm during a daily time window, optionally restricted to certain weekdays." >}}
  {{< card link="/features/anti-affinity/" icon="scale" title="Anti-affinity" subtitle="Make an instance back off automatically while another group is active — for shared GPU/RAM." >}}
  {{< card link="/features/ready-after/" icon="clock" title="Ready after" subtitle="Add a settling delay after an instance reports ready before serving traffic." >}}
  {{< card link="/features/ready-on-start/" icon="lightning-bolt" title="Ready on start" subtitle="Treat a background instance as ready immediately, skipping the health check." >}}
  {{< card link="/features/multiple-groups/" icon="puzzle" title="Multiple groups" subtitle="Let a single instance belong to several groups at once." >}}
{{< /cards >}}
