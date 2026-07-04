---
title: Install Sablier
weight: 20
---

Sablier ships as a Docker image, a prebuilt binary, or you can build it from source.

{{< tabs >}}
{{< tab name="Docker" >}}

Pull the image from Docker Hub or the GitHub Container Registry:

- **Docker Hub** — [sablierapp/sablier](https://hub.docker.com/r/sablierapp/sablier)
- **GitHub Container Registry** — [ghcr.io/sablierapp/sablier](https://github.com/sablierapp/sablier/pkgs/container/sablier)

Run it with a sample [sablier.yaml](https://raw.githubusercontent.com/sablierapp/sablier/main/sablier.sample.yaml):

<!-- x-release-please-start-version -->
```bash
docker run -d -p 10000:10000 \
    -v $PWD/sablier.yaml:/etc/sablier/sablier.yaml sablierapp/sablier:1.14.0
```
<!-- x-release-please-end -->

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

{{< callout type="info" >}}
Next: head to [Getting started](/getting-started/) to wire Sablier up with your provider and reverse proxy.
{{< /callout >}}
