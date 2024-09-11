# Sablier

[![GitHub license](https://img.shields.io/github/license/acouvreur/sablier.svg)](https://github.com/acouvreur/sablier/blob/master/LICENSE)
[![GitHub contributors](https://img.shields.io/github/contributors/acouvreur/sablier.svg)](https://GitHub.com/acouvreur/sablier/graphs/contributors/)
[![GitHub issues](https://img.shields.io/github/issues/acouvreur/sablier.svg)](https://GitHub.com/acouvreur/sablier/issues/)
[![GitHub pull-requests](https://img.shields.io/github/issues-pr/acouvreur/sablier.svg)](https://GitHub.com/acouvreur/sablier/pulls/)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg?style=flat-square)](http://makeapullrequest.com)

[![GoDoc](https://godoc.org/github.com/acouvreur/sablier?status.svg)](http://godoc.org/github.com/acouvreur/sablier)
![Latest Build](https://img.shields.io/github/actions/workflow/status/acouvreur/sablier/build.yml?style=flat-square&branch=main)
![Go Report](https://goreportcard.com/badge/github.com/acouvreur/sablier?style=flat-square)
![Go Version](https://img.shields.io/github/go-mod/go-version/acouvreur/sablier?style=flat-square)
![Latest Release](https://img.shields.io/github/v/release/acouvreur/sablier?style=flat-square&sort=semver)
![Latest PreRelease](https://img.shields.io/github/v/release/acouvreur/sablier?style=flat-square&include_prereleases&sort=semver)

An free and open-source software to start workloads on demand and stop them after a period of inactivity.

![Demo](./docs/assets/img/demo.gif)

Either because you don't want to overload your raspberry pi or because your QA environment gets used only once a week and wastes resources by keeping your workloads up and running, Sablier is a project that might interest you.

## 🎯 Features

- [Supports the following providers](https://acouvreur.github.io/sablier/#/providers/overview)
  - Docker
  - Docker Swarm
  - Kubernetes
- [Supports multiple reverse proxies](https://acouvreur.github.io/sablier/#/plugins/overview)
  - Nginx
  - Traefik
  - Caddy
- Scale up your workload automatically upon the first request
  - [with a themable waiting page](https://acouvreur.github.io/sablier/#/themes)
  - [with a hanging request (hang until service is up)](https://acouvreur.github.io/sablier/#/strategies?id=blocking-strategy)
- Scale your workload to zero automatically after a period of inactivity

## 📝 Documentation

[See the documentation here](https://acouvreur.github.io/sablier/#/)