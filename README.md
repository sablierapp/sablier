# Sablier ![Github Actions](https://img.shields.io/github/workflow/status/acouvreur/sablier/Build?style=flat-square) ![Go Report](https://goreportcard.com/badge/github.com/acouvreur/sablier?style=flat-square) ![Go Version](https://img.shields.io/github/go-mod/go-version/acouvreur/sablier?style=flat-square) ![Latest Release](https://img.shields.io/github/release/acouvreur/sablier/all.svg?style=flat-square)

## Getting started

```bash
docker run -d --name nginx nginx
docker stop nginx
docker run -v /var/run/docker.sock:/var/run/docker.sock -p 10000:10000 ghcr.io/acouvreur/sablier:latest --swarmode=false
curl 'http://localhost:10000/?name=nginx&timeout=1m'
```

## Plugins

## Features

- Support for **Docker** containers
- Support for **Docker Swarm mode**, scale services
- Support for **Kubernetes** Deployments and Statefulsets
- Start your container/service on the first request
- Automatic **scale to zero** after configured timeout upon last request the service received
- Dynamic loading page (cloudflare or grafana cloud style)
- Customize dynamic and loading pages

## Usage

`docker run -v /var/run/docker.sock:/var/run/docker.sock -p 10000:10000 ghcr.io/acouvreur/sablier:latest --swarmode=true`

### CLI

`./sablier --swarmMode=true --kubernetesMode=false`

| Argument         | Value                              | Description                                                                                                                                                  |
| ---------------- | ---------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `swarmMode`      | true,false (default true)          | Enable/Disable swarm mode. Used to determine the scaler implementation.                                                                                      |
| `kubernetesMode` | true,false (default false)         | Enable/Disable Kubernetes mode. Used to determine the scaler implementation.                                                                                 |
| `storagePath`    | path/to/storage/file (default nil) | Enables persistent storage, file will be used to load previous state upon starting and will sync the current content to memory into the file every 5 seconds |

### Docker

- Docker Hub `acouvreur/sablier`
- Ghcr `ghcr.io/acouvreur/sablier`

`docker run -v /var/run/docker.sock:/var/run/docker.sock -p 10000:10000 ghcr.io/acouvreur/sablier:latest --swarmode=true`

### Kubernetes

see <a href="https://github.com/acouvreur/sablier/blob/main/KUBERNETES.md">KUBERNETES.md</a>

### API

```
GET <service_url>:10000/?name=<service_name>&timeout=<timeout>
```

| Query param | Type            | Description                                                             |
| ----------- | --------------- | ----------------------------------------------------------------------- |
| `name`      | `string`        | The docker container name, or the swarm service name                    |
| `timeout`   | `time.Duration` | The duration after which the container/service will be scaled down to 0 |

| Body       | Status code  | Description                                                                    |
| ---------- | ------------ | ------------------------------------------------------------------------------ |
| `started`  | 202 Created  | The container/service is available                                             |
| `starting` | 201 Accepted | The container/service has been scheduled for starting but is not yet available |
