---
title: Kubernetes
weight: 3
since: v1.0.0
aliases:
  - /tutorials/providers/kubernetes/
---

## Introduction

The Kubernetes provider scales [Deployments](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/) and [StatefulSets](https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/) to [zero replicas](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#scaling-a-deployment) when their session expires, and back up when a request needs them. It can also hibernate CloudNativePG Clusters and pause OT-CONTAINER-KIT Redis instances. Sablier runs as a [pod](https://kubernetes.io/docs/concepts/workloads/pods/) in the cluster and calls the Kubernetes API with its [service account](https://kubernetes.io/docs/concepts/security/service-accounts/).

## Install Sablier

{{< tabs >}}
{{< tab name="Helm" >}}

Install the [Helm chart](https://helm.sh/docs/intro/using_helm/) in its own [namespace](https://kubernetes.io/docs/concepts/overview/working-with-objects/namespaces/):

```bash
helm repo add sablier https://sablierapp.github.io/helm-charts
helm install sablier sablier/sablier --namespace sablier --create-namespace -f values.yaml
```

The chart selects the Kubernetes provider and creates the [service account](https://kubernetes.io/docs/concepts/security/service-accounts/) and [RBAC](https://kubernetes.io/docs/reference/access-authn-authz/rbac/) Sablier needs. Set Sablier's [configuration](/reference/cli/) under `config` in your [values file](https://helm.sh/docs/chart_template_guide/values_files/):

{{< example file="kubernetes/values.yaml" region="config" >}}

To manage CloudNativePG Clusters or Redis instances, grant the extra permissions:

{{< example file="kubernetes/values.yaml" region="rbac" >}}

Reverse proxy plugins reach Sablier at `http://sablier-sablier.sablier:10000`: the chart names the [Service](https://kubernetes.io/docs/concepts/services-networking/service/) `sablier-<release>`.

{{< /tab >}}
{{< tab name="Manifests" >}}

Create the [namespace](https://kubernetes.io/docs/concepts/overview/working-with-objects/namespaces/), then apply the [service account](https://kubernetes.io/docs/concepts/security/service-accounts/), [RBAC](https://kubernetes.io/docs/reference/access-authn-authz/rbac/), [Deployment](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/) and [Service](https://kubernetes.io/docs/concepts/services-networking/service/):

```bash
kubectl create namespace sablier
kubectl apply -f sablier.yaml
```

{{< example file="kubernetes/sablier.yaml" >}}

Reverse proxy plugins reach Sablier at `http://sablier.sablier:10000`.

{{< /tab >}}
{{< /tabs >}}

The CloudNativePG and Redis permissions are optional. Without them, or without their [CRDs](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/), Sablier skips those integrations.

{{< callout type="warning" >}}
**`get` and `update` on `deployments/scale` and `statefulsets/scale` are deprecated.** Sablier scales with a `patch` on the scale subresource. If your RBAC only grants `get` and `update` there, as it did for Sablier 1.19 and earlier, Sablier falls back to them and logs a warning once per kind. The fallback can fail when the workload is written concurrently, and it will be removed in the next major release. Grant `patch` on both subresources.
{{< /callout >}}

## Usage example

Opt a [Deployment](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/) in with the `sablierapp.dev/enable` [label](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/), and give it a group:

{{< example file="kubernetes/whoami.yaml" >}}

Then configure your [reverse proxy](/reverse-proxies/) to call Sablier for the `whoami` group in front of the `whoami` [Service](https://kubernetes.io/docs/concepts/services-networking/service/). On the first request, Sablier scales the Deployment to one [replica](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#scaling-a-deployment) and holds the request, or shows a waiting page, until the [pod](https://kubernetes.io/docs/concepts/workloads/pods/) is ready. When the session expires, Sablier scales it back to zero.

{{< callout type="info" >}}
Sablier scales opted-in workloads to zero when it starts. A workload you deploy afterwards keeps running until its first session expires, unless you [stop externally-started instances](/features/stop-external-starts/).
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

Sablier scales [Deployments](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/) and [StatefulSets](https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/) through their [`scale` subresource](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#scaling-a-deployment):

- **Stop** scales to `0`.
- **Start** scales to [`active.replicas`](#scale-instead-of-stop) when set. Otherwise, it uses the replica count of the [instance name](#instance-names) when above `1`, or `1`.

{{< example file="kubernetes/statefulset.yaml" region="sablier" >}}

#### CloudNativePG Clusters {since="v1.13.0"}

Sablier starts and stops [CloudNativePG](https://cloudnative-pg.io/) `Cluster` [custom resources](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) with the [operator](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)'s [declarative hibernation](https://cloudnative-pg.io/documentation/current/declarative_hibernation/):

- **Stop** sets `cnpg.io/hibernation: "on"`. The operator removes the cluster's [pods](https://kubernetes.io/docs/concepts/workloads/pods/) and keeps its [PVCs](https://kubernetes.io/docs/concepts/storage/persistent-volumes/).
- **Start** sets `cnpg.io/hibernation: "off"`, and the operator resumes the cluster.

{{< example file="kubernetes/cnpg-cluster.yaml" >}}

Put the application, its database and any other dependency in the same group, so one request wakes the whole stack and inactivity hibernates all of it.

{{< callout type="info" >}}
Resuming a hibernated cluster takes longer than a scale-up: the PVCs are reattached and PostgreSQL recovers. Make sure your reverse proxy timeouts allow for it.
{{< /callout >}}

#### OT-CONTAINER-KIT Redis instances {since="v1.14.0"}

The [OT-CONTAINER-KIT redis-operator](https://github.com/OT-CONTAINER-KIT/redis-operator) reconciles its [StatefulSets](https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/) back to their replica count, which would undo a scale to zero. Sablier pauses the operator around each scale:

- **Stop** sets the `redis.opstreelabs.in/skip-reconcile: "true"` [annotation](https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/) on the `Redis` resource, then scales its StatefulSet to `0`.
- **Start** scales the StatefulSet back to `1`, then removes the annotation so the operator resumes.

If the scale fails, Sablier clears the annotation immediately, so the operator is never left paused with pods running.

[Label](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/) the `Redis` resource. The operator copies the labels to the StatefulSet it manages:

{{< example file="kubernetes/redis.yaml" >}}

### Instance names

A reverse proxy can target one workload by name instead of a group. The `whoami` Deployment of the usage example is named:

```text
deployment_default_whoami_1
```

The name has four parts, separated by the [delimiter](#configuration) (`_` by default):

- **Kind:** `deployment`, `statefulset` or `cnpgcluster`. Target a Redis instance through its StatefulSet.
- **[Namespace](https://kubernetes.io/docs/concepts/overview/working-with-objects/namespaces/)** of the workload.
- **Name** of the workload.
- **Replicas:** the replica count Sablier scales to on start, since v1.15.0.

Prefer groups: a name ties your proxy configuration to the workload's kind and namespace.

### Labels {since="next"}

On Kubernetes, Sablier keys start with `sablierapp.dev/` instead of `sablier.`:

{{< example file="kubernetes/group.yaml" region="sablier" >}}

- **`enable` must be a [label](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/).** Sablier finds workloads with a [label selector](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors), and selectors can't read [annotations](https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/).
- **Values with commas or colons must be annotations.** Kubernetes [doesn't allow them in labels](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#syntax-and-character-set), so a list of groups or a time window goes in an annotation.
- **Any other key can be either.** If a key is set as both, the annotation wins.

The [label reference](/reference/labels/) lists every key.

{{< details title="Legacy `sablier.*` keys" closed="true" >}}
Earlier releases only read `sablier.*` keys. Sablier still reads them, but they are deprecated on Kubernetes: keys without a prefix [belong to the user](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#syntax-and-character-set) and can clash with your own. If a workload has both forms, the `sablierapp.dev/` key wins.
{{< /details >}}

### Workload settings

Each excerpt below is the `metadata` of a complete manifest in [`examples/kubernetes`](https://github.com/sablierapp/sablier/tree/main/examples/kubernetes).

#### Enable {since="v1.4.0"}

`sablierapp.dev/enable: "true"` opts the workload in. It is required, must be a [label](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/), and any other value is ignored. See the [usage example](#usage-example).

#### Group {since="v1.4.0"}

`sablierapp.dev/group` puts the workload in one or more groups. A session for any of them starts it. Without it, the workload is in the `default` group.

A list of groups contains commas, so set it as an [annotation](https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/):

{{< example file="kubernetes/group.yaml" region="sablier" >}}

[Learn more about groups](/features/groups/).

#### Ready after {since="v1.13.0"}

`sablierapp.dev/ready-after` keeps reporting the workload as not ready for a settling delay after its [pods](https://kubernetes.io/docs/concepts/workloads/pods/) first become ready. Use it when a pod passes its [readiness probe](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/#define-readiness-probes) before it can serve traffic.

{{< example file="kubernetes/ready-after.yaml" region="sablier" >}}

[Learn more about delaying readiness](/features/ready-after/).

#### Ready on start {since="v1.15.0"}

`sablierapp.dev/ready-on-start: "true"` reports the workload ready as soon as Sablier sends the [scale-up](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#scaling-a-deployment), without waiting for its [pods](https://kubernetes.io/docs/concepts/workloads/pods/).

{{< example file="kubernetes/ready-on-start.yaml" region="sablier" >}}

[Learn more about marking ready on start](/features/ready-on-start/).

#### Running hours {since="v1.13.0"}

- `sablierapp.dev/running-hours` keeps the workload running during a daily window.
- `sablierapp.dev/running-days` limits that window to some weekdays, since v1.15.0.

Both values contain colons or commas, so set them as [annotations](https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/):

{{< example file="kubernetes/running-hours.yaml" region="sablier" >}}

The window uses Sablier's local time. With Helm, set the time zone with the `TZ` [environment variable](https://kubernetes.io/docs/tasks/inject-data-application/define-environment-variable-container/):

{{< example file="kubernetes/values.yaml" region="timezone" >}}

[Learn more about running hours](/features/running-hours/).

#### Anti-affinity {since="v1.15.0"}

`sablierapp.dev/anti-affinity` lists groups the workload backs off from: Sablier keeps it idle while any of them has an active session. Set a list of groups as an [annotation](https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/).

{{< example file="kubernetes/anti-affinity.yaml" region="sablier" >}}

[Learn more about anti-affinity](/features/anti-affinity/).

#### Scale instead of stop {since="v1.13.0"}

Keep the workload running when idle, optionally throttled, instead of scaling it to zero:

- `idle.replicas` set to `1` or more keeps that many replicas when idle.
- `idle.cpu` and `idle.memory` set the CPU and memory [limits](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#requests-and-limits) while idle.
- `active.replicas`, `active.cpu` and `active.memory` restore them on the next session.

CPU and memory use [resource quantities](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#resource-units-in-kubernetes), such as `100m`, `2`, `64Mi` or `1Gi`.

{{< example file="kubernetes/scale-mode.yaml" region="sablier" >}}

{{< callout type="warning" >}}
Changing limits triggers a [rolling update](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#rolling-update-deployment): new pods replace the old ones, so the service stays available. Sablier only changes the limits of the [pod template](https://kubernetes.io/docs/concepts/workloads/pods/#pod-templates)'s first container, never its [requests](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#requests-and-limits), so make sure your nodes can schedule the active limits.
{{< /callout >}}

[Learn more about scaling instead of stopping](/features/scale-mode/).

### Readiness

Sablier reports a [Deployment](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/) or [StatefulSet](https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/):

- **ready** when it has at least one desired replica and all of them are ready. With `--provider.kubernetes.ready-on-first-replica`, one ready replica is enough.
- **stopped** when it is scaled to `0`, even while [pods](https://kubernetes.io/docs/concepts/workloads/pods/) are still terminating.
- **starting** otherwise.

A replica is ready when its pod passes its [readiness probe](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/#define-readiness-probes), so give your containers one.

A CloudNativePG Cluster is **stopped** when its `cnpg.io/hibernation` [annotation](https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/) is `"on"`, **ready** when `status.readyInstances` reaches `spec.instances`, and **starting** otherwise.

### Limitations

- Sablier must run as a [pod](https://kubernetes.io/docs/concepts/workloads/pods/) in the cluster it manages: it only uses [in-cluster credentials](https://kubernetes.io/docs/tasks/run-application/access-api-from-pod/).
- [Block I/O throttling](/features/disk-io-limits/) is not available on Kubernetes.
- CPU and memory throttling applies to the first container of the [pod template](https://kubernetes.io/docs/concepts/workloads/pods/#pod-templates) only, and restarts the pods.
- Without the Redis permissions, Sablier logs a warning and scales the [StatefulSet](https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/) anyway; the operator then scales it back up.

## Examples

- [`examples/kubernetes`](https://github.com/sablierapp/sablier/tree/main/examples/kubernetes): every manifest on this page, with instructions to run them.
- [Code-Server + Traefik + Kubernetes Ingress](/guides/code-server-traefik-kubernetes/): a complete setup behind Traefik.
