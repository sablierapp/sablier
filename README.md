# 

![Sablier Banner](https://raw.githubusercontent.com/sablierapp/artwork/refs/heads/main/horizontal/sablier-horizontal-color.png)

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
  - Apache APISIX
  - Caddy
  - Envoy
  - Istio
  - Nginx (NJS Module)
  - Nginx (WASM Module)
  - Traefik
- Scale up your workload automatically upon the first request
  - [with a themable waiting page](https://sablierapp.dev/#/themes)
  - [with a hanging request (hang until service is up)](https://sablierapp.dev/#/strategies?id=blocking-strategy)
- Scale your workload to zero automatically after a period of inactivity

## 📝 Documentation

[See the documentation here](https://sablierapp.dev)
