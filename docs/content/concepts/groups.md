---
title: Groups
weight: 414
---

A **group** is a named handle for one or more [instances](/concepts/how-sablier-works/). Instead of referencing individual container or service names, your reverse proxy references a group, and a [session](/reference/glossary/) for that group starts every instance in it.

## Why groups exist

- **Decouple your proxy from workload names.** The proxy route names a group, not a container. You can rename, replace or scale the underlying workload without touching your proxy configuration.
- **Start related workloads together.** Put an app and the database it needs in the same group, and a single request warms both at once.
- **Target many instances at once.** A group can contain any number of instances; one session request starts them all.

## Assigning a group

Set the `sablier.group` label on the instance:

```yaml
services:
  whoami:
    image: acouvreur/whoami:v1.10.2
    labels:
      - "sablier.enable=true"
      - "sablier.group=demo"
```

{{< callout type="info" >}}
If you enable an instance but do not set `sablier.group`, it joins the group named **`default`**.
{{< /callout >}}

The label lives wherever your provider keeps labels. See [Configuring instances](/concepts/configuring-instances/) for the per-provider syntax (Kubernetes annotations, Proxmox `sablier-group-<name>` tags, and so on).

## Targeting a group from the reverse proxy

Your reverse-proxy plugin names the group to start for a given route. For example, with Caddy:

```Caddyfile
route /whoami {
    sablier http://sablier:10000 {
        group demo
        session_duration 1m
    }
    reverse_proxy whoami:80
}
```

When a request hits `/whoami`, Sablier starts the `demo` group and waits according to the configured [strategy](/concepts/strategies/) before letting the request through. Each reverse proxy expresses this differently; see [Reverse proxies](/tutorials/reverse-proxies/).

## One instance, several groups

An instance can belong to more than one group at the same time; a session for **any** of them starts it. This is useful for shared dependencies. See [Instance groups](/how-to-guides/groups/) for the rules and per-provider examples.

## Groups and sessions

A session is created when a group is requested and is kept alive by ongoing traffic. When it expires after the inactivity window, every instance in the group is stopped. The [Glossary](/reference/glossary/) has the precise definitions of *session* and *group*.
