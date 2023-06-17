![Latest Build](https://img.shields.io/github/actions/workflow/status/acouvreur/sablier/build.yml?style=flat-square&branch=main)![Go Report](https://goreportcard.com/badge/github.com/acouvreur/sablier?style=flat-square) ![Go Version](https://img.shields.io/github/go-mod/go-version/acouvreur/sablier?style=flat-square) ![Latest Release](https://img.shields.io/github/release/acouvreur/sablier/all.svg?style=flat-square)

# Sablier - Scale to Zero

Sablier is a **free** and **open-source** software that can scale your workloads on demand.

![Demo](assets/img/demo.gif)

Your workloads can be a docker container, a kubernetes deployment and more (see [providers](/providers/overview) for the full list).


Sablier is an API that start containers for a given duration.

It provides an integrations with multiple reverse proxies and different loading strategies.

Which allows you to start your containers on demand and shut them down automatically as soon as there's no activity.

## Glossary

I'll use these terms in order to be provider agnostic.

- **Session**: A Session is a set of **instances**
- **Instance**: An instance is either a docker container, docker swarm service, kubernetes deployment or kubernetes statefulset

## Credits

- [Hourglass icons created by Vectors Market - Flaticon](https://www.flaticon.com/free-icons/hourglass)
- [tarampampam/error-pages](https://github.com/tarampampam/error-pages/) for the themes