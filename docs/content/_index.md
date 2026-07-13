---
layout: hextra-home
cascade:
  type: docs
---

<div class="hx:mt-6 hx:mb-6">
{{< hextra/hero-badge link="https://github.com/sablierapp/sablier" >}}
  <div class="hx:w-2 hx:h-2 hx:rounded-full hx:bg-primary-400"></div>
  <span>Free, open source</span>
{{< /hextra/hero-badge >}}
</div>

<div class="hx:mt-6 hx:mb-6">
{{< hextra/hero-headline style="background-image:linear-gradient(90deg,#EA580C 0%,#F59E0B 100%);" >}}
  Scale your workloads&nbsp;<br class="hx:sm:block hx:hidden" />to zero, on demand
{{< /hextra/hero-headline >}}
</div>

<div class="hx:mb-12">
{{< hextra/hero-subtitle >}}
  Start containers on the first request and stop them&nbsp;<br class="hx:sm:block hx:hidden" />automatically when there's no more activity.
{{< /hextra/hero-subtitle >}}
</div>

<div class="hx:mb-6 hx:flex hx:gap-4">
{{< hextra/hero-button text="Get started" link="/tutorials/getting-started/" >}}
{{< hextra/hero-button text="Star on GitHub" link="https://github.com/sablierapp/sablier" style="background:transparent;color:inherit;border:1px solid var(--tw-prose-hr,#d1d5db);" >}}
</div>

<div class="hx:mt-6"></div>

{{< hextra/feature-grid >}}
  {{< hextra/feature-card
    title="Scale to zero"
    icon="lightning-bolt"
    subtitle="Start workloads on the first request and shut them down automatically after a period of inactivity."
  >}}
  {{< hextra/feature-card
    title="Any provider"
    icon="server"
    link="/tutorials/providers/"
    subtitle="Docker, Docker Swarm, Kubernetes, Podman and Proxmox LXC. One API for all of them."
  >}}
  {{< hextra/feature-card
    title="Reverse-proxy native"
    icon="puzzle"
    link="/tutorials/reverse-proxies/"
    subtitle="Drop-in middleware for Traefik, Caddy, Nginx, Envoy, Istio and Apache APISIX."
  >}}
  {{< hextra/feature-card
    title="Loading strategies"
    icon="adjustments"
    link="/concepts/strategies/"
    subtitle="Show a waiting page while it starts (dynamic), or hold the request until it's ready (blocking)."
  >}}
  {{< hextra/feature-card
    title="Scale, don't stop"
    icon="chip"
    link="/how-to-guides/scaling-resources/scale-mode/"
    subtitle="Keep workloads warm with throttled CPU, memory and block I/O instead of full cold starts."
  >}}
  {{< hextra/feature-card
    title="Resource efficient"
    icon="scale"
    link="/concepts/resource-efficiency/"
    subtitle="A minimal footprint that frees CPU, RAM and GPU for the workloads actually in use."
  >}}
{{< /hextra/feature-grid >}}

<div class="hx:mt-16"></div>

## What is Sablier?

Sablier is a **free** and **open-source** API that starts your workloads on demand and stops them when there's no activity. Your workloads can be a Docker container, a Kubernetes deployment, and [more](/tutorials/providers/).

It integrates with multiple reverse proxies and offers different loading [strategies](/concepts/strategies/), so you can start containers on demand and shut them down automatically once they're idle.

New here? See [How Sablier works](/concepts/how-sablier-works/) for the mental model, or the [Glossary](/reference/glossary/) for provider-agnostic definitions of session, instance, label, provider, group, strategy and reverse-proxy plugin.
