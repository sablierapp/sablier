# sablier

![Version: 0.1.0](https://img.shields.io/badge/Version-0.1.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 1.6.0](https://img.shields.io/badge/AppVersion-1.6.0-informational?style=flat-square)

An free and open-source software to start workloads on demand and stop them after a period of inactivity.

## Maintainers

| Name | Email | Url |
| ---- | ------ | --- |
| TODO | <TODO> |  |

## Source Code

* <https://github.com/acouvreur/sablier>

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| deploymentAnnotations | object | `{}` | Annotations for all deployed Deployments |
| deploymentLabels | object | `{}` | Labels for all deployed Deployments |
| deploymentStrategy | object | `{"rollingUpdate":{"maxSurge":"25%","maxUnavailable":"25%"},"type":"RollingUpdate"}` | Deployment strategy for all deployed Deployments |
| image.repository | string | `"acouvreur/sablier"` | Sablier image repository |
| image.tag | string | `""` | Sablier image tag (deafult) appVersion |
| imagePullPolicy | string | `"IfNotPresent"` | Sablier imagePullPolicy |
| livenessProbe | object | `{"failureThreshold":3,"httpGet":{"path":"/healthz","port":10000},"initialDelaySeconds":5,"periodSeconds":5,"successThreshold":1,"timeoutSeconds":1}` | Sablier livenessProbe |
| logLevel | string | `"trace"` | Sablier log level |
| podAnnotations | object | `{}` | Annotations for all deployed pods |
| podLabels | object | `{}` | Labels for all deployed pods |
| readinessProbe | object | `{"failureThreshold":3,"httpGet":{"path":"/healthz","port":10000},"initialDelaySeconds":5,"periodSeconds":5,"successThreshold":1,"timeoutSeconds":1}` | Sablier readinessProbe |
| replicas | int | `1` | Sablier's replicas |
| resources | object | `{}` | Resource limits and requests for sablier |

