# Sablier

![Github Actions](https://img.shields.io/github/workflow/status/acouvreur/sablier/Build?style=flat-square) ![Go Report](https://goreportcard.com/badge/github.com/acouvreur/sablier?style=flat-square) ![Go Version](https://img.shields.io/github/go-mod/go-version/acouvreur/sablier?style=flat-square) ![Latest Release](https://img.shields.io/github/release/acouvreur/sablier/all.svg?style=flat-square)

Sablier is an API that start containers on demand.
It provides an integrations with multiple reverse proxies and different loading strategies.

![Hourglass](./docs/img/hourglass.png)

- [Sablier](#sablier)
  - [Getting started](#getting-started)
  - [Features](#features)
  - [CLI Usage](#cli-usage)
  - [Configuration](#configuration)
  - [Reverse proxies integration plugins](#reverse-proxies-integration-plugins)
    - [Traefik Integration](#traefik-integration)
  - [Kubernetes](#kubernetes)
  - [API](#api)

## Getting started

Binary

```bash
docker run -d --name nginx nginx
docker stop nginx
./sablier start
curl 'http://localhost:10000/?name=nginx&timeout=1m'
```

Docker

```bash
docker run -d --name nginx nginx
docker stop nginx
docker run -v /var/run/docker.sock:/var/run/docker.sock -p 10000:10000 ghcr.io/acouvreur/sablier:latest --swarmode=false
curl 'http://localhost:10000/?name=nginx&timeout=1m'
```

## Features

- Support for **Docker** containers
- Support for **Docker Swarm mode**, scale services
- Support for **Kubernetes** Deployments and Statefulsets
- Start your container/service on the first request
- Automatic **scale to zero** after configured timeout upon last request the service received
- Dynamic loading page (cloudflare or grafana cloud style)
- Customize dynamic and loading pages

## CLI Usage

```
Usage:
  sablier [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  start       Start the Sablier server
  version     Print the version Sablier

Flags:
  -h, --help   help for sablier

Use "sablier [command] --help" for more information about a command.
```

Start options

```
Start the Sablier server

Usage:
  sablier start [flags]

Flags:
  -h, --help                      help for start
      --provider.name string      Provider to use to manage containers [docker swarm kubernetes] (default "docker")
      --server.base-path string   The base path for the API (default "/")
      --server.port int           The server port to use (default 10000)
      --storage.file string       File path to save the state
```

## Configuration

Sablier can be configured in that order:

1. command line arguments
2. environment variable
3. config.yaml file

```yaml
server:
  port: 10000
  basePath: /
storage:
  file: 
provider:
  name: docker # available providers are docker, swarm and kubernetes
```

## Reverse proxies integration plugins

### Traefik Integration

see [Traefik Integration](./plugins/traefik/README.md)


## Kubernetes

see [KUBERNETES.md](https://github.com/acouvreur/sablier/blob/main/KUBERNETES.md)

## API

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

## Credits

[Hourglass icons created by Vectors Market - Flaticon](https://www.flaticon.com/free-icons/hourglass)