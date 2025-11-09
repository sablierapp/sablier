![Sablier Banner](https://raw.githubusercontent.com/sablierapp/artwork/refs/heads/main/horizontal/sablier-horizontal-color.png)

[![Go Report Card](https://goreportcard.com/badge/github.com/sablierapp/sablier)](https://goreportcard.com/report/github.com/sablierapp/sablier)
[![Discord](https://img.shields.io/discord/1298488955947454464?logo=discord&logoColor=5865F2&cacheSeconds=1&link=http%3A%2F%2F)](https://discord.gg/WXYp59KeK9)

A free and open-source software to start workloads on demand and stop them after a period of inactivity.

Think of it a bit like a serverless platform, but for your own servers.

![Demo](./docs/assets/img/demo.gif)

Either because you don't want to overload your raspberry pi or because your QA environment gets used only once a week and wastes resources by keeping your workloads up and running, Sablier is a project that might interest you.

## 🎯 Features

- [Supports the following providers](https://sablierapp.dev/#/providers/overview)
  - Docker
  - Docker Swarm
  - Kubernetes
- [Supports multiple reverse proxies](https://sablierapp.dev/#/plugins/overview)
  - [Apache APISIX](https://github.com/sablierapp/sablier-proxywasm-plugin)
  - [Caddy](https://github.com/sablierapp/sablier-caddy-plugin)
  - [Envoy](https://github.com/sablierapp/sablier-proxywasm-plugin)
  - [Istio](https://github.com/sablierapp/sablier-proxywasm-plugin)
  - Nginx (NJS Module)
  - [Nginx (WASM Module)](https://github.com/sablierapp/sablier-proxywasm-plugin)
  - [Traefik](https://github.com/sablierapp/sablier-proxywasm-plugin)
- Scale up your workload automatically upon the first request
  - [with a themable waiting page](https://sablierapp.dev/#/themes)
  - [with a hanging request (hang until service is up)](https://sablierapp.dev/#/strategies?id=blocking-strategy)
- Scale your workload to zero automatically after a period of inactivity

## 📝 Documentation

[See the documentation here](https://sablierapp.dev)

## Community

Join our Discord server to discuss and get support!

[![Discord](https://img.shields.io/discord/1298488955947454464?logo=discord&logoColor=5865F2&cacheSeconds=1&link=http%3A%2F%2F)](https://discord.gg/WXYp59KeK9)