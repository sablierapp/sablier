# Kubernetes

Sablier assumes that it is deployed within the Kubernetes cluster to use the Kubernetes API internally.

## Use the Kubernetes provider

In order to use the kubernetes provider you can configure the [provider.name](../configuration) property.

<!-- tabs:start -->

#### **File (YAML)**

```yaml
provider:
  name: kubernetes
```

#### **CLI**

```bash
sablier start --provider.name=kubernetes
```

#### **Environment Variable**

```bash
SABLIER_PROVIDER_NAME=kubernetes
```

<!-- tabs:end -->

!> **Ensure that Sablier has the necessary roles!**

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
```

?> The `postgresql.cnpg.io` rule is optional. When the CloudNativePG CRD is absent, Sablier simply skips CloudNativePG discovery.

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

?> Kubernetes uses the Pod healthcheck to check if the Pod is up and running. So the provider has a native healthcheck support.

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

?> Resuming a hibernated cluster takes longer than a simple scale-up (PVC reattachment and PostgreSQL recovery). Make sure your reverse-proxy timeouts allow for it.