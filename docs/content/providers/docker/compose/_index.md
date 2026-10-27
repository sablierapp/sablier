---
title: Docker Compose
description: How the Docker provider works with Docker Compose projects.
weight: 1
---

Sablier manages containers created by `docker compose` like any other container: opt them in with `sablier.*` labels. The Docker provider also reads the metadata `docker compose` writes on each container (the `com.docker.compose.*` labels), so Compose features keep working when Sablier starts a stack on demand.

{{< section-cards >}}
