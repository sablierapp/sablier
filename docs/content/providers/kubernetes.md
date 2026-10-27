---
title: Kubernetes
weight: 3
aliases:
  - /tutorials/providers/kubernetes/
---

{{< since "v1.0.0" >}}

## Introduction

The Kubernetes provider scales Deployments and StatefulSets to zero replicas when their session expires, and back up when a request needs them. It can also hibernate CloudNativePG Clusters and pause OT-CONTAINER-KIT Redis instances. Sablier runs as a pod in the cluster and calls the Kubernetes API with its service account.

## Install Sablier

{{< tabs >}}
{{< tab name="Helm" >}}

Install the chart in its own namespace:

```bash
helm repo add sablier https://sablierapp.github.io/helm-charts
helm install sablier sablier/sablier --namespace sablier --create-namespace -f values.yaml
```

The chart selects the Kubernetes provider and creates the service account and RBAC Sablier needs. Set Sablier's [configuration](/reference/cli/) under `config`:

{{< example file="kubernetes/values.yaml" region="config" >}}

To manage CloudNativePG Clusters or Redis instances, grant the extra permissions:

{{< example file="kubernetes/values.yaml" region="rbac" >}}

Reverse proxy plugins reach Sablier at `http://sablier-sablier.sablier:10000`: the chart names the Service `sablier-<release>`.

{{< /tab >}}
{{< tab name="Manifests" >}}

Create the namespace, then apply the service account, RBAC, Deployment and Service:

```bash
kubectl create namespace sablier
kubectl apply -f sablier.yaml
```

{{< example file="kubernetes/sablier.yaml" >}}

Reverse proxy plugins reach Sablier at `http://sablier.sablier:10000`.

{{< /tab >}}
{{< /tabs >}}

The CloudNativePG and Redis permissions are optional. Without them, or without their CRDs, Sablier skips those integrations.

## Usage example

Opt a Deployment in with the `sablierapp.dev/enable` label, and give it a group:

{{< example file="kubernetes/whoami.yaml" >}}

Then configure your [reverse proxy](/reverse-proxies/) to call Sablier for the `whoami` group in front of the `whoami` Service. On the first request, Sablier scales the Deployment to one replica and holds the request, or shows a waiting page, until the pod is ready. When the session expires, Sablier scales it back to zero.

{{< callout type="info" >}}
Sablier scales opted-in workloads to zero when it starts. A workload you deploy afterwards keeps running until its first session expires, unless you [stop externally-started instances](/how-to-guides/lifecycle/auto-stop-externally-started/).
{{< /callout >}}

## Provider reference

### Configuration

| Option | Default | Description |
|---|---|---|
| [`--provider.kubernetes.delimiter`](/reference/cli/#opt-provider-kubernetes-delimiter) | `_` | The separator in [instance names](#instance-names). Prefer `.` or `/`; `_` is kept for backward compatibility. |
| [`--provider.kubernetes.ready-on-first-replica`](/reference/cli/#opt-provider-kubernetes-ready-on-first-replica) | `false` | Report a workload ready as soon as one replica is ready, instead of all of them. Since v1.16.0. |
| [`--provider.kubernetes.qps`](/reference/cli/#opt-provider-kubernetes-qps) | `5` | Client-side limit of Kubernetes API requests per second. |
| [`--provider.kubernetes.burst`](/reference/cli/#opt-provider-kubernetes-burst) | `10` | Requests allowed above that limit in a burst. |

Every `--provider.*` option that is not provider-specific also applies; see the [CLI reference](/reference/cli/).

### Workloads

#### Deployments and StatefulSets

Sablier scales Deployments and StatefulSets through their `scale` subresource. On stop, it scales to `0`. On start, it scales to [`sablierapp.dev/active.replicas`](#scale-instead-of-stop) when set, otherwise to the replica count of the [instance name](#instance-names) when above `1`, otherwise to `1`.

{{< example file="kubernetes/statefulset.yaml" region="sablier" >}}

#### CloudNativePG Clusters

{{< since "v1.13.0" >}}

Sablier starts and stops [CloudNativePG](https://cloudnative-pg.io/) `Cluster` resources with the operator's [declarative hibernation](https://cloudnative-pg.io/documentation/current/declarative_hibernation/):

- **Stop** sets `cnpg.io/hibernation: "on"`. The operator removes the cluster's pods and keeps its PVCs.
- **Start** sets `cnpg.io/hibernation: "off"`, and the operator resumes the cluster.

{{< example file="kubernetes/cnpg-cluster.yaml" >}}

Put the application, its database and any other dependency in the same group, so one request wakes the whole stack and inactivity hibernates all of it.

{{< callout type="info" >}}
Resuming a hibernated cluster takes longer than a scale-up: the PVCs are reattached and PostgreSQL recovers. Make sure your reverse proxy timeouts allow for it.
{{< /callout >}}

#### OT-CONTAINER-KIT Redis instances

{{< since "v1.14.0" >}}

The [OT-CONTAINER-KIT redis-operator](https://github.com/OT-CONTAINER-KIT/redis-operator) reconciles its StatefulSets back to their replica count, which would undo a scale to zero. Sablier pauses the operator around each scale:

- **Stop** sets `redis.opstreelabs.in/skip-reconcile: "true"` on the `Redis` resource, then scales its StatefulSet to `0`.
- **Start** scales the StatefulSet back to `1`, then removes the annotation so the operator resumes.

If the scale fails, Sablier clears the annotation immediately, so the operator is never left paused with pods running.

Label the `Redis` resource. The operator copies the labels to the StatefulSet it manages:

{{< example file="kubernetes/redis.yaml" >}}

### Instance names

A reverse proxy can target one workload by name instead of a group. Sablier names each workload:

```text
<kind><delimiter><namespace><delimiter><name><delimiter><replicas>
```

`<kind>` is `deployment`, `statefulset` or `cnpgcluster`. A Redis instance is targeted through its StatefulSet. With the default delimiter, the `whoami` Deployment of the usage example is `deployment_default_whoami_1`; with `--provider.kubernetes.delimiter=.`, it is `deployment.default.whoami.1`.

`<replicas>` is the replica count Sablier scales to on start, since v1.15.0. Prefer groups: a name ties your proxy configuration to the workload's kind and namespace.

### Labels

{{< since "next" >}}

Sablier settings use the `sablierapp.dev/` prefix on Kubernetes: replace `sablier.` in the [label reference](/reference/labels/) with `sablierapp.dev/`. For example, `sablier.idle.replicas` becomes `sablierapp.dev/idle.replicas`. This applies to Deployments, StatefulSets, CloudNativePG Clusters and Redis instances.

Set `sablierapp.dev/enable` as a **label**: Sablier discovers workloads with a label selector, which cannot match annotations. Set every other key as a label or an annotation. Kubernetes label values are limited to 63 characters of `[A-Za-z0-9._-]`, so values with commas or colons, like a list of groups or a time window, must be annotations. When a key is both a label and an annotation, the annotation wins.

Sablier still reads the legacy `sablier.*` keys, which earlier releases require. They are deprecated on Kubernetes because a key without a prefix is [private to the user](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#syntax-and-character-set) and can clash with your own conventions. When a workload has both forms, the `sablierapp.dev/` key wins over the `sablier.` key from the same source.

### Workload settings

Each excerpt below is the `metadata` of a complete manifest in [`examples/kubernetes`](https://github.com/sablierapp/sablier/tree/main/examples/kubernetes).

#### Enable

{{< since "v1.4.0" >}}

`sablierapp.dev/enable: "true"` opts the workload in. It is required, must be a label, and any other value is ignored. See the [usage example](#usage-example).

#### Group

{{< since "v1.4.0" >}}

`sablierapp.dev/group` assigns the workload to one or more groups. A session for any of them starts it. A workload without a group is in the `default` group. A list of groups contains commas, so set it as an annotation:

{{< example file="kubernetes/group.yaml" region="sablier" >}}

[Learn more about groups](/how-to-guides/groups/).

#### Ready after

{{< since "v1.13.0" >}}

`sablierapp.dev/ready-after` keeps reporting the workload as not ready for a settling delay after its pods first become ready. Use it when a pod passes its readiness probe before it can serve traffic.

{{< example file="kubernetes/ready-after.yaml" region="sablier" >}}

[Learn more about delaying readiness](/how-to-guides/readiness/ready-after/).

#### Ready on start

{{< since "v1.15.0" >}}

`sablierapp.dev/ready-on-start: "true"` reports the workload ready as soon as Sablier sends the scale-up, without waiting for its pods.

{{< example file="kubernetes/ready-on-start.yaml" region="sablier" >}}

[Learn more about marking ready on start](/how-to-guides/readiness/ready-on-start/).

#### Running hours

{{< since "v1.13.0" >}}

`sablierapp.dev/running-hours` keeps the workload running during a daily window, and `sablierapp.dev/running-days` (since v1.15.0) limits the window to some weekdays. Both values contain colons or commas, so set them as annotations:

{{< example file="kubernetes/running-hours.yaml" region="sablier" >}}

The window uses Sablier's local time. With Helm, set the time zone with the `TZ` environment variable:

{{< example file="kubernetes/values.yaml" region="timezone" >}}

[Learn more about running hours](/how-to-guides/lifecycle/running-hours/).

#### Anti-affinity

{{< since "v1.15.0" >}}

`sablierapp.dev/anti-affinity` lists groups the workload backs off from: Sablier keeps it idle while any of them has an active session. Set a list of groups as an annotation.

{{< example file="kubernetes/anti-affinity.yaml" region="sablier" >}}

[Learn more about anti-affinity](/how-to-guides/anti-affinity/).

#### Scale instead of stop

{{< since "v1.13.0" >}}

With `sablierapp.dev/idle.replicas` set to `1` or more, Sablier keeps the workload running when idle instead of scaling it to zero, and can throttle it. `idle.cpu` and `idle.memory` set the CPU and memory **limits** while idle; `active.replicas`, `active.cpu` and `active.memory` restore them on the next session. CPU and memory use [resource quantities](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#resource-units-in-kubernetes), such as `100m`, `2`, `64Mi` or `1Gi`.

{{< example file="kubernetes/scale-mode.yaml" region="sablier" >}}

Sablier changes the limits of the pod template's **first container**, which triggers a rolling restart: new pods replace the old ones, so the service stays available. Requests are not changed, so make sure your nodes can schedule the active limits.

[Learn more about scaling instead of stopping](/how-to-guides/scaling-resources/scale-mode/).

### Readiness

Sablier reports a Deployment or StatefulSet:

- **ready** when it has at least one desired replica and all of them are ready. With `--provider.kubernetes.ready-on-first-replica`, one ready replica is enough.
- **stopped** when it is scaled to `0`, even while pods are still terminating.
- **starting** otherwise.

A replica is ready when its pod passes its [readiness probe](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/), so give your containers one.

A CloudNativePG Cluster is **stopped** when its `cnpg.io/hibernation` annotation is `"on"`, **ready** when `status.readyInstances` reaches `spec.instances`, and **starting** otherwise.

### Limitations

- Sablier must run as a pod in the cluster it manages: it only uses in-cluster credentials.
- [Block I/O throttling](/how-to-guides/scaling-resources/scale-blkio/) is not available on Kubernetes.
- CPU and memory throttling applies to the first container of the pod template only, and restarts the pods.
- Without the Redis permissions, Sablier logs a warning and scales the StatefulSet anyway; the operator then scales it back up.

## Examples

- [`examples/kubernetes`](https://github.com/sablierapp/sablier/tree/main/examples/kubernetes): every manifest on this page, with instructions to run them.
- [Code-Server + Traefik + Kubernetes Ingress](/guides/code-server-traefik-kubernetes/): a complete setup behind Traefik.
