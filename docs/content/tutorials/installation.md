---
title: Install Sablier
description: Install Sablier as a Docker image, Helm chart, prebuilt binary, or from source.
weight: 20
---

Sablier ships as a Docker image, a Helm chart for Kubernetes, a prebuilt binary, or you can build it from source.

{{< tabs >}}
{{< tab name="Docker" >}}

Pull the image from Docker Hub or the GitHub Container Registry:

- **Docker Hub**: [sablierapp/sablier](https://hub.docker.com/r/sablierapp/sablier)
- **GitHub Container Registry**: [ghcr.io/sablierapp/sablier](https://github.com/sablierapp/sablier/pkgs/container/sablier)

**With Docker Compose**, copy this into a `compose.yaml` and run `docker compose up -d`:

<!-- x-release-please-start-version -->
```yaml
services:
  sablier:
    image: sablierapp/sablier:1.14.0
    command:
      - start
      - --provider.name=docker
    ports:
      - "10000:10000"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
```
<!-- x-release-please-end -->

**With `docker run`**, using a sample [sablier.yaml](https://raw.githubusercontent.com/sablierapp/sablier/main/sablier.sample.yaml):

<!-- x-release-please-start-version -->
```bash
docker run -d -p 10000:10000 \
    -v $PWD/sablier.yaml:/etc/sablier/sablier.yaml sablierapp/sablier:1.14.0
```
<!-- x-release-please-end -->

The `docker.sock` mount is what lets Sablier start and stop your Docker containers.

{{< /tab >}}
{{< tab name="Helm" >}}

Deploy Sablier to a Kubernetes cluster with the official [Helm chart](https://github.com/sablierapp/helm-charts/tree/main/charts/sablier).

Add the Sablier Helm repository:

```bash
helm repo add sablierapp https://sablierapp.github.io/helm-charts
helm repo update
```

Install Sablier:

```bash
helm install sablier sablierapp/sablier
```

See the [chart documentation](https://github.com/sablierapp/helm-charts/tree/main/charts/sablier) for the available `values.yaml` options (provider, strategy, resources, and more).

{{< /tab >}}
{{< tab name="Binary" >}}

Download the latest binary from the [releases](https://github.com/sablierapp/sablier/releases) page and run it:

```bash
./sablier --help
```

{{< /tab >}}
{{< tab name="From source" >}}

```bash
git clone git@github.com:sablierapp/sablier.git
cd sablier
make
# Output will vary depending on your platform
./sablier_draft_linux-amd64
```

{{< /tab >}}
{{< /tabs >}}
