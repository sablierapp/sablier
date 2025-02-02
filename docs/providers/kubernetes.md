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
    app.kubernetes.io/name: whoami
    sablierapp.dev/enable: "true"
    sablierapp.dev/group: mygroup
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: whoami
  template:
    metadata:
      labels:
        app.kubernetes.io/name: whoami
    spec:
      containers:
      - name: whoami
        image: acouvreur/whoami:v1.10.2
```

## How does Sablier knows when a deployment is ready?

Sablier checks for the deployment replicas. As soon as the current replicas matches the wanted replicas, then the deployment is considered `ready`.

?> Kubernetes uses the Pod healthcheck to check if the Pod is up and running. So the provider has a native healthcheck support.