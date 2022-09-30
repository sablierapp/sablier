<h1 align="center">
  <img src="https://blog.alterway.fr/images/traefik.logo.png" alt="Traefik Ondemand Plugin" width="200">
  <br>Traefik Ondemand Service<br>
</h1>

<h4 align="center">Traefik Ondemand Service for <a href="https://github.com/acouvreur/traefik-ondemand-plugin/plugins/traefik">traefik-ondemand-plugin</a> to control containers and services.</h4>

<p align="center">
  <a href="https://github.com/acouvreur/traefik-ondemand-service/actions">
    <img src="https://img.shields.io/github/workflow/status/acouvreur/traefik-ondemand-service/Build?style=flat-square" alt="Github Actions">
  </a>
  <a href="https://goreportcard.com/report/github.com/acouvreur/traefik-ondemand-service">
    <img src="https://goreportcard.com/badge/github.com/acouvreur/traefik-ondemand-service?style=flat-square">
  </a>
  <img src="https://img.shields.io/github/go-mod/go-version/acouvreur/traefik-ondemand-service?style=flat-square">
  <a href="https://github.com/acouvreur/traefik-ondemand-service/releases">
    <img src="https://img.shields.io/github/release/acouvreur/traefik-ondemand-service/all.svg?style=flat-square">
  </a>
  <a href="https://github.com/acouvreur/traefik-ondemand-service/releases">
    <img src="https://img.shields.io/docker/image-size/acouvreur/traefik-ondemand-service?style=flat-square">
  </a>
</p>

## Features

- Support for Docker containers
- Support for Docker swarm mode, scale services
- Support for Kubernetes Deployments and Statefulsets
- Start your container/service on the first request
- Dynamic loading page (cloudflare or grafana cloud style)
- Automatic scale to zero after configured timeout upon last request the service received
- Support container/service healthcheck and will not redirect until service is healthy

## Usage

### CLI

`./traefik-ondemand-service --swarmMode=true --kubernetesMode=false`

| Argument         | Value                              | Description                                                                                                                                                  |
| ---------------- | ---------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `swarmMode`      | true,false (default true)          | Enable/Disable swarm mode. Used to determine the scaler implementation.                                                                                      |
| `kubernetesMode` | true,false (default false)         | Enable/Disable Kubernetes mode. Used to determine the scaler implementation.                                                                                 |
| `storagePath`    | path/to/storage/file (default nil) | Enables persistent storage, file will be used to load previous state upon starting and will sync the current content to memory into the file every 5 seconds |

### Docker

- Docker Hub `acouvreur/traefik-ondemand-service`
- Ghcr `ghcr.io/acouvreur/traefik-ondemand-service`

`docker run -v /var/run/docker.sock:/var/run/docker.sock -p 10000:10000 ghcr.io/acouvreur/traefik-ondemand-service:latest --swarmode=true`

### Kubernetes

see <a href="https://github.com/acouvreur/traefik-ondemand-service/blob/main/KUBERNETES.md">KUBERNETES.md</a>

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
