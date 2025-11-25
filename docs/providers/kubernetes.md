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
PROVIDER_NAME=kubernetes
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
```

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

## How does Sablier know when a deployment is ready?

Sablier checks for the deployment replicas. As soon as the current replicas match the wanted replicas, then the deployment is considered `ready`.

?> Kubernetes uses the Pod healthcheck to check if the Pod is up and running. So the provider has native healthcheck support.

## Configuration Options

### Kubernetes-specific Settings

```yaml
provider:
  name: kubernetes
  kubernetes:
    qps: 5            # K8S API QPS limit (default: 5)
    burst: 10         # K8S API burst limit (default: 10)
    delimiter: "_"    # Namespace/resource delimiter (default: "_")
```

#### QPS and Burst

These settings control client-side throttling for Kubernetes API requests:

- **QPS (Queries Per Second)**: Maximum sustained request rate
- **Burst**: Maximum burst of requests allowed

For large clusters with many deployments, you may need to increase these values:

```yaml
provider:
  kubernetes:
    qps: 50
    burst: 100
```

#### Delimiter

The delimiter separates parts of the resource identifier:

```yaml
# With delimiter="_" (default)
sablier.group: namespace_deployment_name

# With delimiter="/"
sablier.group: namespace/deployment/name

# With delimiter="."
sablier.group: namespace.deployment.name
```

### Auto-stop on Startup

```yaml
provider:
  auto-stop-on-startup: true
```

When enabled, Sablier will scale to 0 all deployments/statefulsets with `sablier.enable=true` label that have non-zero replicas but are not registered in an active session when Sablier starts.

## Resource Labels

| Label | Required | Description | Example |
|-------|----------|-------------|---------|
| `sablier.enable` | Yes | Enable Sablier management | `"true"` |
| `sablier.group` | Yes | Logical group name | `myapp` |

**Important:** In Kubernetes, label values must be strings (use quotes for boolean/numeric values).

## Supported Resources

Sablier supports the following Kubernetes resources:

### Deployments

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: myapp
  labels:
    sablier.enable: "true"
    sablier.group: mygroup
spec:
  replicas: 0
  # ...
```

### StatefulSets

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: myapp
  labels:
    sablier.enable: "true"
    sablier.group: mygroup
spec:
  replicas: 0
  # ...
```

## RBAC Requirements

Sablier requires specific permissions to function:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: sablier
rules:
  - apiGroups: ["apps", ""]
    resources: ["deployments", "statefulsets"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["apps", ""]
    resources: ["deployments/scale", "statefulsets/scale"]
    verbs: ["get", "list", "watch", "patch", "update"]
```

### Minimal Permissions

If you want to restrict Sablier to specific namespaces, use a `Role` instead of `ClusterRole`:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: sablier
  namespace: my-namespace
# ... same rules as above
```

## Full Example

See the [Kubernetes provider example](../../examples/kubernetes/) for a complete, working setup with all manifests.

## Scaling Behavior

- Deployments/StatefulSets start with 0 replicas
- On first request, Sablier scales to the last known replica count (default: 1)
- When session expires, Sablier scales back to 0
- Kubernetes scheduler handles pod placement
- Pod readiness probes determine when the deployment is ready

## Deployment Strategies

Sablier respects Kubernetes deployment strategies:

### Rolling Update

```yaml
spec:
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 0
      maxSurge: 1
```

### Recreate

```yaml
spec:
  strategy:
    type: Recreate
```

## Limitations

- Sablier must run inside the Kubernetes cluster (uses InClusterConfig)
- Requires appropriate RBAC permissions
- Does not support DaemonSets (scaling to 0 not applicable)
- Does not support plain ReplicaSets (use Deployments instead)

## Troubleshooting

### Permission Denied Errors

Check RBAC configuration:

```bash
kubectl describe clusterrole sablier
kubectl describe clusterrolebinding sablier
kubectl auth can-i get deployments --as=system:serviceaccount:sablier-system:sablier
```

### Deployment Not Scaling

1. Check Sablier logs:
   ```bash
   kubectl logs -l app=sablier -n sablier-system
   ```

2. Verify labels:
   ```bash
   kubectl get deployment <name> -o yaml | grep -A 5 labels
   ```

3. Check Sablier can access the deployment:
   ```bash
   kubectl get deployments --all-namespaces -l sablier.enable=true
   ```

### Pods Not Starting

Check pod events:
```bash
kubectl describe deployment <name>
kubectl get events --sort-by='.lastTimestamp'
```

### Rate Limiting

If you see rate limiting errors in logs, increase QPS and burst:

```yaml
provider:
  kubernetes:
    qps: 50
    burst: 100
```