# Providers

## What is a Provider?

A Provider is how Sablier can interact with your instances.

A Provider typically have the following capabilities:
- Start an instance
- Stop an instance
- Get the current status of an instance
- Listen for instance lifecycle events (started, stopped)

## Available providers

| Provider                                | Name                      | Details                                                          |
| --------------------------------------- | ------------------------- | ---------------------------------------------------------------- |
| [Docker](/providers/docker)             | `docker`                  | Stop and start **containers** on demand                          |
| [Docker Swarm](/providers/docker_swarm) | `docker_swarm` or `swarm` | Scale down to zero and up **services** on demand                 |
| [Kubernetes](/providers/kubernetes)     | `kubernetes`              | Scale down and up **deployments** and **statefulsets** on demand |
| [Podman](/providers/podman)             | `podman`                  | [See #70](https://github.com/acouvreur/sablier/issues/70)        |
| [ECS](/providers/ec2)                   | `ecs`                     | [See #116](https://github.com/acouvreur/sablier/issues/116)      |
| [Systemd](/providers/systemd)           | `systemd`                 | [See #148](https://github.com/acouvreur/sablier/issues/148)      |

*Your Provider is not on the list? [Open an issue to request the missing provider here!](https://github.com/acouvreur/sablier/issues/new?assignees=&labels=enhancement%2C+provider&projects=&template=instance-provider-request.md&title=Add+%60%5BPROVIDER%5D%60+provider)*

[See the active issues about the providers](https://github.com/acouvreur/sablier/issues?q=is%3Aopen+is%3Aissue+label%3Aprovider)