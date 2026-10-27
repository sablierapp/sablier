---
title: Nomad
weight: 7
aliases:
  - /tutorials/providers/nomad/
---

This tutorial connects Sablier to [HashiCorp Nomad](https://developer.hashicorp.com/nomad). You will select the Nomad provider, point Sablier at the Nomad API with an ACL token, register a job task group so Sablier can manage it, and confirm Sablier knows when the task group is ready. The Nomad provider scales task groups to zero and back through the Nomad scaling API.

{{< callout type="warning" >}}
The Nomad provider is **experimental**. Its configuration and behavior can change in a minor release. Report problems on [GitHub](https://github.com/sablierapp/sablier/issues).
{{< /callout >}}

## Select the Nomad provider

Set the `provider.name` property to `nomad` and point it at your Nomad API. See the [CLI reference](/reference/cli/) for every Nomad flag.

{{< tabs >}}
{{< tab name="File (YAML)" >}}

```yaml
provider:
  name: nomad
  nomad:
    address: "http://nomad.service.consul:4646"
    token: "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
    namespace: "default"
```

{{< /tab >}}
{{< tab name="CLI" >}}

```bash
sablier start \
  --provider.name=nomad \
  --provider.nomad.address=http://nomad.service.consul:4646 \
  --provider.nomad.token=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx \
  --provider.nomad.namespace=default
```

{{< /tab >}}
{{< tab name="Environment Variable" >}}

```bash
SABLIER_PROVIDER_NAME=nomad
SABLIER_PROVIDER_NOMAD_ADDRESS=http://nomad.service.consul:4646
SABLIER_PROVIDER_NOMAD_TOKEN=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
SABLIER_PROVIDER_NOMAD_NAMESPACE=default
```

{{< /tab >}}
{{< /tabs >}}

Every setting is optional. A setting left empty falls back to the standard environment variables of the Nomad client, so a Sablier that already runs with `NOMAD_ADDR`, `NOMAD_TOKEN`, `NOMAD_NAMESPACE` and `NOMAD_REGION` needs no further configuration. The built-in defaults are `http://127.0.0.1:4646`, no token and the `default` namespace.

TLS is configured with the same environment variables as the `nomad` CLI: `NOMAD_CACERT`, `NOMAD_CAPATH`, `NOMAD_CLIENT_CERT`, `NOMAD_CLIENT_KEY`, `NOMAD_TLS_SERVER_NAME` and `NOMAD_SKIP_VERIFY`.

## Create an ACL token

When ACLs are enabled, Sablier needs a token that can list and read the jobs of the namespace, scale their task groups, and submit jobs. Sablier submits a job only to change its count while an active deployment blocks the scaling API. Create a policy file `sablier.hcl`:

```hcl
namespace "default" {
  capabilities = ["list-jobs", "read-job", "scale-job", "submit-job"]
}
```

Apply it and create the token:

```bash
nomad acl policy apply -description "Sablier" sablier sablier.hcl
nomad acl token create -name sablier -policy sablier -type client
```

Use the **Secret ID** of the token as `provider.nomad.token`.

## Register a task group

For Sablier to work, it needs to know which task groups to scale. Register a task group by opting in with the `sablier.enable` key in its `meta` block. Start the group with `count = 0` so it only runs on demand:

```hcl
job "whoami" {
  datacenters = ["dc1"]

  group "web" {
    count = 0

    meta = {
      "sablier.enable" = "true"
      "sablier.group"  = "mygroup"
    }

    network {
      port "http" {
        to = 80
      }
    }

    task "server" {
      driver = "docker"

      config {
        image = "traefik/whoami"
        ports = ["http"]
      }

      service {
        name     = "whoami"
        port     = "http"
        provider = "nomad"

        check {
          type     = "http"
          path     = "/"
          interval = "5s"
          timeout  = "2s"
        }
      }
    }
  }
}
```

Write `meta` as a map, `meta = { ... }`. The label names contain dots, and HCL does not accept quoted argument names in a `meta { ... }` block.

The `meta` of the job applies to every task group, and the `meta` of a task group overrides it. A job with a single task group can therefore carry the `sablier.*` keys at the job level.

Every label of the [labels reference](/reference/labels/) is read from `meta` with the same name, for example `"sablier.ready-after" = "30s"`.

{{< callout type="warning" >}}
A task group with a `scaling` block must allow `min = 0`, otherwise Nomad rejects the scale-to-zero request.
{{< /callout >}}

## Instance naming

Sablier identifies a task group as `<job ID>/<task group name>`, for example `whoami/web`. Use that name in the `names` parameter of your reverse proxy configuration:

```yaml
# Traefik middleware example
http:
  middlewares:
    my-sablier:
      plugin:
        sablier:
          sablierUrl: http://sablier:10000
          names: whoami/web
```

A request with only the job ID is rejected with an error that lists the task groups of the job. Dispatched and periodic child jobs contain a `/` in their ID; the task group name is everything after the last `/`.

## Runnable example

A complete, runnable example is available in [`examples/nomad`](https://github.com/sablierapp/sablier/tree/main/examples/nomad). It runs a Nomad agent in development mode and Sablier with Docker Compose, so it does not need a Nomad installation.

## Confirm when the task group is ready

Sablier reads the task group `count` and the allocations of the current job version. A `service` task group gets a deployment on every scale-up, and Nomad marks each allocation healthy once its tasks run and its service checks pass for `min_healthy_time`. Sablier reports the task group as ready when every desired allocation is healthy.

| Nomad state | Sablier status |
|---|---|
| `count = 0` | Stopped |
| Allocations pending, or running but not healthy yet | Starting |
| Every desired allocation running and healthy | Ready |
| Running allocations without deployment tracking (no `update` block, batch jobs) | Ready |
| Every allocation of a `batch` task group completed | Completed |
| Only failed allocations left | Error |
| The job is stopped with `nomad job stop` | Error |

{{< callout type="info" >}}
Sablier scales task groups and never stops a job, so it leaves a job stopped with `nomad job stop` stopped. Requests for it get an error until you run `nomad job start`.
{{< /callout >}}

## Limitations

- One Sablier server manages one Nomad namespace, selected by `provider.nomad.namespace`.
- `system` and `sysbatch` jobs have no `count` and cannot be scaled.
- The CPU and memory profiles of [scale mode](/how-to-guides/scaling-resources/scale-mode/) are not supported; the replica profiles are.

## Run Sablier as a Nomad job

Sablier itself can run as a Nomad job. Pass the token through a template so it never appears in the job specification:

```hcl
job "sablier" {
  datacenters = ["dc1"]

  group "sablier" {
    network {
      port "http" {
        to = 10000
      }
    }

    task "sablier" {
      driver = "docker"

      config {
        image = "sablierapp/sablier:{{< version >}}"
        args  = ["start", "--provider.name=nomad"]
        ports = ["http"]
      }

      env {
        NOMAD_ADDR      = "http://${attr.unique.network.ip-address}:4646"
        NOMAD_NAMESPACE = "default"
      }

      template {
        data        = "NOMAD_TOKEN={{ with nomadVar \"nomad/jobs/sablier\" }}{{ .token }}{{ end }}"
        destination = "secrets/env"
        env         = true
      }

      service {
        name     = "sablier"
        port     = "http"
        provider = "nomad"
      }
    }
  }
}
```

## Use with Traefik

When a task group scales to 0, its service is deregistered, and Traefik removes the router that Nomad or Consul tags create. Requests then get a 404 and never reach the Sablier middleware.

- With the Traefik Nomad provider, set `providers.nomad.allowEmptyServices=true` so the router stays.
- With the Traefik Consul Catalog provider, or with Consul Connect services, add a fallback router with the lowest priority in the file provider:

```yaml
http:
  routers:
    whoami-wake:
      rule: Host(`whoami.example.com`)
      priority: 1
      service: noop@internal
      middlewares:
        - sablier-whoami
      entryPoints:
        - https
      tls: {}
```

While the task group runs, its own router has a higher priority and handles the requests. Add the Sablier middleware to that router too, so requests renew the session. While the task group is at 0, the fallback router catches the requests and Sablier starts the group.

Set `sablier.ready-after` to at least the refresh interval of the Traefik provider (15 seconds for Consul Catalog). Otherwise the waiting page can send the user on before Traefik has the router again, and the request lands on `noop@internal`.

{{< callout type="info" >}}
An uptime monitor that requests the application through Traefik renews the session on every check, so the task group never scales to 0. Monitor the Sablier `/health` endpoint and the Nomad job instead.
{{< /callout >}}

## Feature support

Which Sablier features work with the Nomad provider. A ≈ mark means the feature works, but you configure it differently than on other providers. See the [compatibility matrix](/reference/compatibility/) to compare every provider.

{{< provider-features "nomad" >}}
