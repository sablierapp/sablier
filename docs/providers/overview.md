# Providers

## What is a Provider?

A Provider is how Sablier can interact with your instances.

A Provider typically have the following capabilities:
- Start an instance
- Stop an instance
- Get the current status of an instance
- Listen for instance lifecycle events (started, stopped)

## Available providers

|       | Provider                                    | Name        | Details                                                  |
| :---: | --------------------------------------- | -------------- | -------------------------------------------------------- |
|       | [Docker](/providers/docker)             | `docker`       | Stop and start containers on demand                      |
|       | [Docker Swarm](/providers/docker_swarm) | `docker_swarm` | Scale down to zero and up services on demand             |
|       | [Docker](/providers/kubernetes)         | `kubernetes`   | Scale down and up deployments and statefulsets on demand |
|       | [Podman](/providers/podman)             | `podman`       | Work in progress                                         |
|       | [EC2](/providers/ec2)                   | `ec2`          | Work in progress                                         |
|       | [Systemd](/providers/systemd)           | `systemd`      | Work in progress                                         |

*Your Provider is not on the list? [Open an issue to request the missing provider here!](https://github.com/acouvreur/sablier/issues/new?assignees=&labels=enhancement%2C+provider&projects=&template=instance-provider-request.md&title=Add+%60%5BPROVIDER%5D%60+provider)*