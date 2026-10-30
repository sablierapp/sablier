# Kubernetes Examples

Runnable manifests for the [Kubernetes provider](https://sablierapp.dev/providers/kubernetes/).
The documentation includes these files directly, so what you read is what you
apply. The workloads use [sablierapp/mimic](https://github.com/sablierapp/mimic),
a configurable web-server built for testing purposes.

## Files

| File | Shows |
|---|---|
| `values.yaml` | Helm values for the `sablier/sablier` chart |
| `sablier.yaml` | Sablier without Helm: ServiceAccount, RBAC, Deployment and Service |
| `leader-election.yaml` | Two Sablier replicas in active/passive mode, on top of `sablier.yaml` |
| `whoami.yaml` | A Deployment and its Service, the starting point for every other example |
| `group.yaml` | One workload in two groups (annotation, because of the comma) |
| `ready-after.yaml` | A settling delay after the pods are ready |
| `ready-on-start.yaml` | Ready as soon as the scale-up is sent |
| `running-hours.yaml` | Kept warm on weekdays during office hours |
| `anti-affinity.yaml` | Forced idle while the `streaming` group is active |
| `scale-mode.yaml` | Kept at one throttled replica instead of zero |
| `statefulset.yaml` | A StatefulSet in the `whoami` group |
| `cnpg-cluster.yaml` | A CloudNativePG Cluster in the `whoami` group |
| `redis.yaml` | An OT-CONTAINER-KIT Redis instance in the `whoami` group |

Lines between `# region <name>` and `# endregion` are the excerpts the
documentation shows; Kubernetes ignores the markers.

## Run it

You need a cluster (for example [k3d](https://k3d.io) or [kind](https://kind.sigs.k8s.io)), `kubectl` and `helm`.

```bash
helm repo add sablier https://sablierapp.github.io/helm-charts
helm install sablier sablier/sablier --namespace sablier --create-namespace -f values.yaml

kubectl apply -f whoami.yaml
```

Request the `whoami` group through the Sablier API, as a reverse proxy plugin
would. The request opens a one-minute session:

```bash
kubectl port-forward --namespace sablier svc/sablier-sablier 10000 &
curl "http://localhost:10000/api/strategies/blocking?group=whoami&session_duration=1m"
```

One minute after the last request, Sablier scales `whoami` to zero
(`kubectl get deployment whoami` shows `0/0`). Run the same `curl` again:
Sablier scales it back up and answers once the pod is ready.

Apply any other file the same way to try that setting.
