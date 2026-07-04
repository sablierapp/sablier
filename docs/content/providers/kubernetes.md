---
title: Kubernetes
weight: 3
---

Sablier assumes that it is deployed within the Kubernetes cluster to use the Kubernetes API internally.

## Use the Kubernetes provider

In order to use the kubernetes provider you can configure the [provider.name](/configuration/) property.

{{< tabs >}}
{{< tab name="File (YAML)" >}}

```yaml
provider:
  name: kubernetes
```

{{< /tab >}}
{{< tab name="CLI" >}}

```bash
sablier start --provider.name=kubernetes
```

{{< /tab >}}
{{< tab name="Environment Variable" >}}

```bash
SABLIER_PROVIDER_NAME=kubernetes
```

{{< /tab >}}
{{< /tabs >}}

{{< callout type="warning" >}}
**Ensure that Sablier has the necessary roles!**
{{< /callout >}}

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: sablier
rules:
  - apiGroups:
      - apps
      - ""
    resources:
      - deployments
      - statefulsets
    verbs:
      - get     # Retrieve info about specific dep
      - list    # Events
      - watch   # Events
  - apiGroups:
      - apps
      - ""
    resources:
      - deployments/scale
      - statefulsets/scale
    verbs:
      - patch   # Scale up and down
      - update  # Scale up and down
      - get     # Retrieve info about specific dep
      - list    # Events
      - watch   # Events
  # Only required if you manage CloudNativePG Clusters (see below).
  - apiGroups:
      - postgresql.cnpg.io
    resources:
      - clusters
    verbs:
      - get     # Retrieve info about a specific cluster
      - list    # Discovery and events
      - watch   # Events
      - patch   # Toggle the hibernation annotation
  # Only required if you manage OT-CONTAINER-KIT Redis instances (see below).
  - apiGroups:
      - redis.redis.opstreelabs.in
    resources:
      - redis
    verbs:
      - get     # Resolve the Redis CR owner of a StatefulSet
      - patch   # Toggle the skip-reconcile annotation
```

{{< callout type="info" >}}
The `postgresql.cnpg.io` and `redis.redis.opstreelabs.in` rules are optional. Sablier skips those integrations gracefully when the CRDs are absent.
{{< /callout >}}

## Register Deployments

For Sablier to work, it needs to know which deployments to scale up and down.

You have to register your deployments by opting-in with labels.


```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: whoami
  labels:
    app: whoami
    sablier.enable: "true"
    sablier.group: mygroup
spec:
  selector:
    matchLabels:
      app: whoami
  template:
    metadata:
      labels:
        app: whoami
    spec:
      containers:
      - name: whoami
        image: acouvreur/whoami:v1.10.2
```

## How does Sablier knows when a deployment is ready?

Sablier checks for the deployment replicas. As soon as the current replicas matches the wanted replicas, then the deployment is considered `ready`.

{{< callout type="info" >}}
Kubernetes uses the Pod healthcheck to check if the Pod is up and running. So the provider has a native healthcheck support.
{{< /callout >}}

## Configure with labels or annotations

On Kubernetes, `sablier.*` keys can be set either as a **label** or as an **annotation**, with one exception: `sablier.enable` must always be a label (see the note below). This applies to Deployments, StatefulSets, CloudNativePG Clusters and OT-CONTAINER-KIT Redis instances.

Annotations are useful because Kubernetes **label values are restricted** (max 63 characters, only `[A-Za-z0-9._-]`, no commas or colons). Some Sablier values cannot be expressed as labels and must use annotations, for example:

- `sablier.group` with multiple comma-separated groups (e.g. `team-a,team-b`)
- `sablier.running-hours` (e.g. `09:00-18:00`) — the colon is invalid in a label value
- `sablier.running-days` (e.g. `Mon,Tue,Wed,Thu,Fri`)

When the same key is present as both a label and an annotation, the **annotation takes precedence**.

{{< callout type="warning" >}}
`sablier.enable` must be set as a **label**. Workload discovery relies on a server-side label selector, which cannot match annotations. All other keys work as labels or annotations.
{{< /callout >}}

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: whoami
  labels:
    app: whoami
    sablier.enable: "true"       # must be a label
  annotations:
    sablier.group: "team-a,team-b"          # comma is invalid as a label value
    sablier.running-hours: "09:00-18:00"    # colon is invalid as a label value
    sablier.running-days: "Mon,Tue,Wed,Thu,Fri"
spec:
  # ...existing spec...
```

## Register CloudNativePG Clusters

Sablier can also start and stop [CloudNativePG](https://cloudnative-pg.io/) `Cluster` resources. Instead of scaling a replica count, Sablier toggles CloudNativePG's [declarative hibernation](https://cloudnative-pg.io/documentation/current/declarative_hibernation/) annotation:

- **Stop** sets `cnpg.io/hibernation: "on"` — the operator scales the cluster down and removes its workload while keeping the PVCs.
- **Start** sets `cnpg.io/hibernation: "off"` — the operator resumes the cluster.

Opt-in with the same labels used for deployments and statefulsets:

```yaml
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata:
  name: opencell-db
  labels:
    sablier.enable: "true"
    sablier.group: opencell
spec:
  instances: 3
  storage:
    size: 1Gi
```

This makes it possible to put an application, its Keycloak and its database in a single `sablier.group`, so that a single request wakes up the whole stack and inactivity hibernates all of it.

### Cluster readiness

A CloudNativePG Cluster is considered:

- `stopped` when the `cnpg.io/hibernation` annotation is `"on"`;
- `ready` when `status.readyInstances` is greater than or equal to `spec.instances`;
- `starting` otherwise.

{{< callout type="info" >}}
Resuming a hibernated cluster takes longer than a simple scale-up (PVC reattachment and PostgreSQL recovery). Make sure your reverse-proxy timeouts allow for it.
{{< /callout >}}

## Register OT-CONTAINER-KIT Redis instances

Sablier can scale to zero StatefulSets managed by the [OT-CONTAINER-KIT redis-operator](https://github.com/OT-CONTAINER-KIT/redis-operator). The operator continuously reconciles its StatefulSets back to the desired replica count, so a plain scale-to-zero is immediately undone. Sablier works around this by toggling the operator's own pause mechanism:

- **Stop** sets `redis.opstreelabs.in/skip-reconcile: "true"` on the Redis CR, then scales the StatefulSet to 0.
- **Start** scales the StatefulSet back to 1, then removes the annotation so the operator resumes normal reconciliation.

If the scale fails, the annotation is cleared immediately so the operator is never left paused with pods still running.

Opt-in by adding the standard Sablier labels to the Redis CR. The operator propagates them to the StatefulSet it manages, so no extra labelling is needed:

```yaml
apiVersion: redis.redis.opstreelabs.in/v1beta2
kind: Redis
metadata:
  name: myapp-redis
  labels:
    sablier.enable: "true"
    sablier.group: myapp
spec:
  kubernetesConfig:
    image: quay.io/opstree/redis:v7.0.12
```

This makes it straightforward to group a Redis instance with the rest of an application stack so that a single request wakes everything up and inactivity shuts it all down together.

{{< callout type="info" >}}
The `redis.redis.opstreelabs.in` RBAC rule (see above) is required for Sablier to patch the skip-reconcile annotation on the Redis CR. If the rule is absent or the CRD is not installed, Sablier logs a warning and scales the StatefulSet anyway — the operator will reconcile replicas back, but no other harm is done.
{{< /callout >}}