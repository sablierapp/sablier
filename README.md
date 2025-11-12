<!-- omit in toc -->
# 

![Sablier Banner](https://raw.githubusercontent.com/sablierapp/artwork/refs/heads/main/horizontal/sablier-horizontal-color.png)

[![Go Report Card](https://goreportcard.com/badge/github.com/sablierapp/sablier)](https://goreportcard.com/report/github.com/sablierapp/sablier)
[![Discord](https://img.shields.io/discord/1298488955947454464?logo=discord&logoColor=5865F2&cacheSeconds=1&link=http%3A%2F%2F)](https://discord.gg/WXYp59KeK9)

Free and open-source software that starts workloads on demand and stops them after a period of inactivity.

![Demo](./docs/assets/img/demo.gif)

Whether you don't want to overload your Raspberry Pi, or your QA environment is used only once a week and wastes resources by keeping workloads running, Sablier might be the solution you need.

- [Installation](#installation)
  - [Use the Docker image](#use-the-docker-image)
  - [Use the binary distribution](#use-the-binary-distribution)
  - [Compile your binary from the sources](#compile-your-binary-from-the-sources)
  - [Use the Helm Chart](#use-the-helm-chart)
- [Configuration](#configuration)
  - [Configuration File](#configuration-file)
  - [Environment Variables](#environment-variables)
  - [Arguments](#arguments)
- [Providers](#providers)
  - [Docker](#docker)
  - [Docker Swarm](#docker-swarm)
  - [Podman](#podman)
  - [Kubernetes](#kubernetes)
- [Usage with Reverse Proxies](#usage-with-reverse-proxies)
  - [Apache APISIX](#apache-apisix)
  - [Caddy](#caddy)
  - [Envoy](#envoy)
  - [Istio](#istio)
  - [Nginx](#nginx)
  - [Traefik](#traefik)
- [📝 Documentation](#-documentation)
- [Community](#community)


## Installation

You can install Sablier using one of the following methods:

- [Use the Docker image](#use-the-docker-image)
- [Use the binary distribution](#use-the-binary-distribution)
- [Compile your binary from the sources](#compile-your-binary-from-the-sources)
- [Use the Helm Chart](#use-the-helm-chart)

### Use the Docker image

![Docker Pulls](https://img.shields.io/docker/pulls/sablierapp/sablier)
![Docker Image Size (tag)](https://img.shields.io/docker/image-size/sablierapp/sablier/1.10.1)

- **Docker Hub**: [sablierapp/sablier](https://hub.docker.com/r/sablierapp/sablier)
- **GitHub Container Registry**: [ghcr.io/sablierapp/sablier](https://github.com/sablierapp/sablier/pkgs/container/sablier)
  
Choose one of the Docker images and run it with a sample configuration file:

- [sablier.yaml](https://raw.githubusercontent.com/sablierapp/sablier/main/sablier.sample.yaml)

```bash
docker run -d -p 10000:10000 -v sablier.yaml:/etc/sablier/sablier.yaml sablierapp/sablier:1.10.1
```

### Use the binary distribution

Grab the latest binary from the [releases](https://github.com/sablierapp/sablier/releases) page and run it:

```bash
./sablier --help
```

### Compile your binary from the sources

```bash
git clone git@github.com:sablierapp/sablier.git
cd sablier
make
# Output will change depending on your distro
./sablier_draft_linux-amd64
```

### Use the Helm Chart

<img src="./docs/assets/img/helm.png" alt="Helm" width="100" align="right" />

Deploy Sablier to your Kubernetes cluster using the official Helm chart for production-ready deployments.

Add the Sablier Helm repository:

```bash
helm repo add sablierapp https://sablierapp.github.io/helm-charts
helm repo update
```

Install Sablier:

```bash
helm install sablier sablierapp/sablier
```

📚 **[Full Documentation](https://github.com/sablierapp/helm-charts/tree/main/charts/sablier)** | 💻 **[Chart Repository](https://github.com/sablierapp/helm-charts)**

---

## Configuration

There are three ways to configure Sablier:

1. [In a configuration file](#configuration-file)
2. [As environment variables](#environment-variables)
3. [As command-line arguments](#arguments)

Configuration sources are evaluated in the order listed above.

If no value is provided for a given option, a default value is used.

### Configuration File

At startup, Sablier searches for a configuration file named sablier.yml (or sablier.yaml) in:

- `/etc/sablier/`
- `$XDG_CONFIG_HOME/`
- `$HOME/.config/`
- `.` *(the working directory)*

You can override this using the `configFile` argument.

```bash
sablier --configFile=path/to/myconfigfile.yml
```

```yaml
provider:
  # Provider to use to manage containers (docker, swarm, kubernetes)
  name: docker 
server:
  # The server port to use
  port: 10000 
  # The base path for the API
  base-path: /
storage:
  # File path to save the state (default stateless)
  file:
sessions:
  # The default session duration (default 5m)
  default-duration: 5m
  # The expiration checking interval. 
  # Higher duration gives less stress on CPU. 
  # If you only use sessions of 1h, setting this to 5m is a good trade-off.
  expiration-interval: 20s
logging:
  level: debug
strategy:
  dynamic:
    # Custom themes folder, will load all .html files recursively (default empty)
    custom-themes-path:
    # Show instances details by default in waiting UI
    show-details-by-default: false
    # Default theme used for dynamic strategy (default "hacker-terminal")
    default-theme: hacker-terminal
    # Default refresh frequency in the HTML page for dynamic strategy
    default-refresh-frequency: 5s
  blocking:
    # Default timeout used for blocking strategy (default 1m)
    default-timeout: 1m
```

### Environment Variables

Environment variables follow the same structure as the configuration file. For example:

```yaml
strategy:
  dynamic:
    custom-themes-path: /my/path
```

becomes

```bash
STRATEGY_DYNAMIC_CUSTOM_THEMES_PATH=/my/path
```

### Arguments

To list all available arguments:

```bash
sablier --help

# or

docker run sablierapp/sablier[:version] --help
# e.g.: docker run sablierapp/sablier:1.10.1 --help
```

Command-line arguments follow the same structure as the configuration file. For example:

```yaml
strategy:
  dynamic:
    custom-themes-path: /my/path
```

becomes

```bash
sablier start --strategy.dynamic.custom-themes-path /my/path
```

<!--
## Reference
TODO: Add link to full auto-generated reference
-->

## Providers

### Docker

<img src="./docs/assets/img/docker.svg" alt="Docker" width="100" align="right" />

Sablier integrates seamlessly with Docker Engine to manage container lifecycle based on demand.

**Features:**
- Start and stop containers automatically
- Scale containers based on HTTP traffic
- Works with Docker Compose deployments

📚 **[Full Documentation](https://sablierapp.dev/#/providers/docker)**

---

### Docker Swarm

<img src="./docs/assets/img/docker_swarm.png" alt="Docker Swarm" width="100" align="right" />

Sablier supports Docker Swarm mode for managing services across a cluster of Docker engines.

**Features:**
- Scale Swarm services on demand
- Distributed scaling across multiple nodes
- Seamless integration with Docker Swarm orchestration

📚 **[Full Documentation](https://sablierapp.dev/#/providers/docker_swarm)**

---

### Podman

<img src="./docs/assets/img/podman.png" alt="Podman" width="100" align="right" />

Sablier works with Podman, the daemonless container engine, providing the same dynamic scaling capabilities as Docker.

**Features:**
- Rootless container management
- Docker-compatible API integration
- Seamless migration from Docker

📚 **[Full Documentation](https://sablierapp.dev/#/providers/docker)**

---

### Kubernetes

<img src="./docs/assets/img/kubernetes.png" alt="Kubernetes" width="100" align="right" />

Sablier provides native Kubernetes support for managing deployments, scaling workloads dynamically.

**Features:**
- Scale Kubernetes deployments and statefulsets
- Works with any Kubernetes cluster
- Label-based workload selection

📚 **[Full Documentation](https://sablierapp.dev/#/providers/kubernetes)**

## Usage with Reverse Proxies

### Apache APISIX

<img src="./docs/assets/img/apacheapisix.png" alt="Apache APISIX" width="100" align="right" />

Sablier integrates with Apache APISIX through a Proxy-WASM plugin, enabling dynamic scaling for your services.

**Quick Start:**
1. Install the Sablier Proxy-WASM plugin
2. Configure APISIX routes with Sablier plugin settings
3. Define your scaling labels on target services

📚 **[Full Documentation](https://github.com/sablierapp/sablier-proxywasm-plugin)** | 💻 **[Plugin Repository](https://github.com/sablierapp/sablier-proxywasm-plugin)**

---

### Caddy

<img src="./docs/assets/img/caddy.png" alt="Caddy" width="100" align="right" />

Sablier provides a native Caddy module for seamless integration with Caddy v2.

**Quick Start:**
1. Build Caddy with the Sablier module using `xcaddy`
2. Add Sablier directives to your Caddyfile
3. Configure dynamic scaling rules

📚 **[Full Documentation](https://github.com/sablierapp/sablier-caddy-plugin)** | 💻 **[Plugin Repository](https://github.com/sablierapp/sablier-caddy-plugin)**

---

### Envoy

<img src="./docs/assets/img/envoy.png" alt="Envoy" width="100" align="right" />

Sablier integrates with Envoy Proxy through a Proxy-WASM plugin for high-performance dynamic scaling.

**Quick Start:**
1. Deploy the Sablier Proxy-WASM plugin
2. Configure Envoy HTTP filters
3. Set up scaling labels on your workloads

📚 **[Full Documentation](https://github.com/sablierapp/sablier-proxywasm-plugin)** | 💻 **[Plugin Repository](https://github.com/sablierapp/sablier-proxywasm-plugin)**

---

### Istio

<img src="./docs/assets/img/istio.png" alt="Istio" width="100" align="right" />

Sablier works with Istio service mesh using the Proxy-WASM plugin for intelligent traffic management.

**Quick Start:**
1. Install the Sablier Proxy-WASM plugin in your Istio mesh
2. Configure EnvoyFilter resources
3. Annotate your services with Sablier labels

📚 **[Full Documentation](https://github.com/sablierapp/sablier-proxywasm-plugin)** | 💻 **[Plugin Repository](https://github.com/sablierapp/sablier-proxywasm-plugin)**

---

### Nginx

<img src="./docs/assets/img/nginx.svg" alt="Nginx" width="100" align="right" />

Sablier integrates with Nginx through a WASM module, bringing dynamic scaling to your Nginx deployments.

**Quick Start:**
1. Build Nginx with WASM support
2. Load the Sablier Proxy-WASM plugin
3. Configure Nginx locations with Sablier directives

📚 **[Full Documentation](https://github.com/sablierapp/sablier-proxywasm-plugin)** | 💻 **[Plugin Repository](https://github.com/sablierapp/sablier-proxywasm-plugin)**

---

### Traefik

<img src="./docs/assets/img/traefik.png" alt="Traefik" width="100" align="right" />

Sablier provides a powerful middleware plugin for Traefik, the cloud-native application proxy.

**Quick Start:**
1. Add the Sablier plugin to your Traefik static configuration
2. Create Sablier middleware in your dynamic configuration
3. Apply the middleware to your routes

📚 **[Full Documentation](https://github.com/sablierapp/sablier-traefik-plugin)** | 💻 **[Plugin Repository](https://github.com/sablierapp/sablier-traefik-plugin)**


## 📝 Documentation

[See the documentation here](https://sablierapp.dev)

## Community

Join our Discord server to discuss and get support!

[![Discord](https://img.shields.io/discord/1298488955947454464?logo=discord&logoColor=5865F2&cacheSeconds=1&link=http%3A%2F%2F)](https://discord.gg/WXYp59KeK9)