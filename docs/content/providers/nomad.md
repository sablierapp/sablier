---
title: Nomad
weight: 7
since: next
aliases:
  - /tutorials/providers/nomad/
---

## Introduction

The Nomad provider scales [task groups](https://developer.hashicorp.com/nomad/docs/job-specification/group) to a [count](https://developer.hashicorp.com/nomad/docs/job-specification/group#count) of zero when their session expires, and back up when a request needs them. Each task group of a [job](https://developer.hashicorp.com/nomad/docs/job-specification/job) is a separate instance. Sablier calls the [Nomad HTTP API](https://developer.hashicorp.com/nomad/api-docs) with an [ACL token](https://developer.hashicorp.com/nomad/docs/secure/acl), and follows job changes on its [event stream](https://developer.hashicorp.com/nomad/api-docs/events).

{{< callout type="warning" >}}
The Nomad provider is **experimental**. Its configuration and behavior can change in a minor release. Report problems on [GitHub](https://github.com/sablierapp/sablier/issues).
{{< /callout >}}

## Install Sablier

Sablier runs wherever it can reach the Nomad API: as a Nomad job, or next to the cluster.

When your cluster has [ACLs](https://developer.hashicorp.com/nomad/docs/secure/acl) enabled, give Sablier a token first. Write an [ACL policy](https://developer.hashicorp.com/nomad/docs/other-specifications/acl-policy#namespace-rules) for the [namespace](https://developer.hashicorp.com/nomad/docs/govern/namespaces) of your jobs:

{{< example file="nomad/sablier-policy.hcl" >}}

Apply it and [create a client token](https://developer.hashicorp.com/nomad/commands/acl/token/create):

```bash
nomad acl policy apply -description "Sablier" sablier sablier-policy.hcl
nomad acl token create -name sablier -policy sablier -type client
```

Sablier uses the **Secret ID** of the token.

{{< tabs >}}
{{< tab name="Nomad job" >}}

Store the token in a [Nomad variable](https://developer.hashicorp.com/nomad/docs/concepts/variables#task-access-to-variables) that the `sablier` job can read, then run the job:

```bash
nomad var put nomad/jobs/sablier token=<secret ID>
nomad job run sablier.nomad.hcl
```

{{< example file="nomad/sablier.nomad.hcl" >}}

The [template](https://developer.hashicorp.com/nomad/docs/job-specification/template) reads the token into `NOMAD_TOKEN`, so it never appears in the job. The job uses the [Docker driver](https://developer.hashicorp.com/nomad/docs/deploy/task-driver/docker).

Reverse proxy plugins reach Sablier on port `10000` of the node that runs the job. The job also registers the `sablier` [service](https://developer.hashicorp.com/nomad/docs/job-specification/service) with Nomad.

{{< /tab >}}
{{< tab name="Outside Nomad" >}}

Run Sablier with the Nomad provider and the address of the Nomad API, for example with Docker Compose:

{{< example file="nomad/compose.yml" region="sablier" >}}

Add `--provider.nomad.token` when your cluster has ACLs. Reverse proxy plugins reach Sablier at `http://<sablier host>:10000`.

{{< /tab >}}
{{< /tabs >}}

## Usage example

Opt a [task group](https://developer.hashicorp.com/nomad/docs/job-specification/group) in with `sablier.enable` in its [meta block](https://developer.hashicorp.com/nomad/docs/job-specification/meta), and register it with a [count](https://developer.hashicorp.com/nomad/docs/job-specification/group#count) of zero:

{{< example file="nomad/whoami.nomad.hcl" >}}

Then configure your [reverse proxy](/reverse-proxies/) to call Sablier for the `demo` group in front of the `whoami` [service](https://developer.hashicorp.com/nomad/docs/job-specification/service). On the first request, Sablier scales the task group to one [allocation](https://developer.hashicorp.com/nomad/docs/glossary#allocation) and holds the request, or shows a waiting page, until the allocation is healthy. When the session expires, Sablier scales it back to zero.

The demo runs the busybox web server of the Nomad image with the [`raw_exec` driver](https://developer.hashicorp.com/nomad/docs/deploy/task-driver/raw_exec), so it works on a [development agent](https://developer.hashicorp.com/nomad/commands/agent#dev). Use your own driver and image.

{{< callout type="info" >}}
Sablier scales opted-in task groups to zero when it starts. A job you register afterwards keeps running until its first session expires, unless you [stop externally-started instances](/how-to-guides/lifecycle/auto-stop-externally-started/).
{{< /callout >}}

### With Traefik

A task group at zero has no allocation, so Nomad removes its [service](https://developer.hashicorp.com/nomad/docs/job-specification/service). Traefik then drops its router, and requests never reach the Sablier middleware. Keep a route:

- **With the Traefik Nomad provider, set [`allowEmptyServices`](https://doc.traefik.io/traefik/reference/install-configuration/providers/hashicorp/nomad/#opt-providers-nomad-allowEmptyServices) to `true`.** Traefik then keeps the router of a service without instances.
- **With Consul Catalog or Consul Connect, add a fallback router.** Give it the lowest [priority](https://doc.traefik.io/traefik/reference/routing-configuration/http/routing/rules-and-priority/), in the [file provider](https://doc.traefik.io/traefik/reference/install-configuration/providers/others/file/):

{{< example file="nomad/traefik.yml" region="fallback" >}}

While the task group runs, its own router has a higher priority. Add the Sablier middleware to it too, so requests renew the session.

Set [`sablier.ready-after`](#ready-after) to at least the [refresh interval](https://doc.traefik.io/traefik/reference/install-configuration/providers/hashicorp/consul-catalog/#opt-providers-consulCatalog-refreshInterval) of the Traefik provider, 15 seconds by default. Otherwise the waiting page can send the user on before Traefik knows the new allocation.

{{< callout type="info" >}}
An uptime monitor that requests the application through Traefik renews the session on every check, so the task group never scales to zero. Monitor the Sablier `/health` endpoint and the Nomad job instead.
{{< /callout >}}

## Provider reference

### Configuration

| Option | Default | Description |
|---|---|---|
| [`--provider.nomad.address`](/reference/cli/#opt-provider-nomad-address) | `NOMAD_ADDR`, then `http://127.0.0.1:4646` | The URL of the Nomad HTTP API. |
| [`--provider.nomad.token`](/reference/cli/#opt-provider-nomad-token) | `NOMAD_TOKEN` | The Secret ID of the ACL token. Leave it empty when ACLs are disabled. |
| [`--provider.nomad.namespace`](/reference/cli/#opt-provider-nomad-namespace) | `NOMAD_NAMESPACE`, then `default` | The [namespace](https://developer.hashicorp.com/nomad/docs/govern/namespaces) of the jobs Sablier manages. |
| [`--provider.nomad.region`](/reference/cli/#opt-provider-nomad-region) | `NOMAD_REGION`, then the region of the agent | The [region](https://developer.hashicorp.com/nomad/docs/glossary#regions) Sablier sends its requests to. |

An empty option falls back to the [environment variable](https://developer.hashicorp.com/nomad/commands#connection-environment-variables) of the `nomad` CLI. A Sablier that runs with `NOMAD_ADDR` and `NOMAD_TOKEN` needs no option.

{{< details title="TLS and HTTP basic authentication" closed="true" >}}
Sablier reads TLS settings from the [environment variables](https://developer.hashicorp.com/nomad/commands#mutual-tls-mtls-environment-variables) of the `nomad` CLI only:

- `NOMAD_CACERT` or `NOMAD_CAPATH`: the CA certificate, or a folder of them.
- `NOMAD_CLIENT_CERT` and `NOMAD_CLIENT_KEY`: the client certificate and its key.
- `NOMAD_TLS_SERVER_NAME`: the server name to verify.
- `NOMAD_SKIP_VERIFY`: skip the certificate verification.

`NOMAD_HTTP_AUTH` sets HTTP basic authentication, as `user:password`.
{{< /details >}}

With [tracing](/how-to-guides/advanced/observability/tracing/) enabled, every Nomad API call is a child span. The event stream is one span for the whole connection.

Every `--provider.*` option that is not provider-specific also applies; see the [CLI reference](/reference/cli/).

### Workloads

#### Task groups

Sablier scales each [task group](https://developer.hashicorp.com/nomad/docs/job-specification/group) of a [job](https://developer.hashicorp.com/nomad/docs/job-specification/job) on its own, with the [Scale Task Group API](https://developer.hashicorp.com/nomad/api-docs/jobs#scale-task-group):

- **Stop** sets the [count](https://developer.hashicorp.com/nomad/docs/job-specification/group#count) to `0`, or to [`idle.replicas`](#scale-instead-of-stop) when set.
- **Start** sets the count to [`active.replicas`](#scale-instead-of-stop) when set, or to `1`.

Start never restores the count the task group had before. Set `sablier.active.replicas` to run more than one [allocation](https://developer.hashicorp.com/nomad/docs/glossary#allocation).

- **Sablier never starts a stopped job.** A job stopped with [`nomad job stop`](https://developer.hashicorp.com/nomad/commands/job/stop) stays stopped, and requests for it get an error. Run [`nomad job start`](https://developer.hashicorp.com/nomad/commands/job/start) to start it again.
- **An active deployment blocks scaling.** Nomad rejects a scale request while a [deployment](https://developer.hashicorp.com/nomad/docs/glossary#deployment) of the job is active. Sablier then registers the job again with the new count, which needs the `submit-job` capability.

{{< details title="When the registration fails" closed="true" >}}
Sablier registers the job only if it did not change since Sablier read it. If someone updates the job in between, the registration fails and Sablier reports the error.
{{< /details >}}

### Instance names

A reverse proxy can target one task group by name instead of a group. The `web` task group of the `whoami` job in the usage example is named:

```text
whoami/web
```

The name has two parts, separated by `/`:

- **Job ID** of the [job](https://developer.hashicorp.com/nomad/docs/job-specification/job).
- **Name** of the [task group](https://developer.hashicorp.com/nomad/docs/job-specification/group).

The [namespace](https://developer.hashicorp.com/nomad/docs/govern/namespaces) is not part of the name: one Sablier manages one namespace. Prefer groups: a name ties your proxy configuration to the structure of the job.

{{< details title="Job IDs with a slash" closed="true" >}}
Dispatched and periodic child jobs have a `/` in their ID. Sablier splits the name on the last `/`, so the task group name is everything after it.

A name with only the job ID fails, and the error lists the task groups of the job.
{{< /details >}}

### Labels

On Nomad, Sablier settings are keys of a [meta block](https://developer.hashicorp.com/nomad/docs/job-specification/meta), on the job or on a task group. In this job, the job `meta` enables both task groups and puts them in the `shop` group:

{{< example file="nomad/job-meta.nomad.hcl" region="job" >}}

The `meta` of the `admin` task group moves it to the `back-office` group:

{{< example file="nomad/job-meta.nomad.hcl" region="group" >}}

- **Use the map syntax `meta = { ... }`.** HCL accepts keys with dots only as quoted map keys, see [meta keys with dots](https://developer.hashicorp.com/nomad/docs/job-specification/meta#meta-keys-with-dots).
- **Job `meta` applies to every task group of the job.** Each task group is still a separate instance.
- **Task group `meta` wins over job `meta`.** Sablier merges them key by key, the way Nomad does for tasks.
- **Task `meta` is ignored.** Sablier reads the job and the task group only.

The keys have the same names as in the [label reference](/reference/labels/), which lists every key.

### Workload settings

Each excerpt below is the `meta` of a task group in a complete job in [`examples/nomad`](https://github.com/sablierapp/sablier/tree/main/examples/nomad).

#### Enable {since="next"}

`"sablier.enable" = "true"` opts the task group in. It is required, and any other value is ignored. Set it in the job `meta` to enable every task group of the job. See the [usage example](#usage-example).

#### Group {since="next"}

`sablier.group` puts the task group in one or more groups, separated by commas. A session for any of them starts it. Without it, the task group is in the `default` group.

{{< example file="nomad/group.nomad.hcl" region="sablier" >}}

[Learn more about groups](/how-to-guides/groups/).

#### Ready after {since="next"}

`sablier.ready-after` keeps reporting the task group as not ready for a settling delay after its [allocations](https://developer.hashicorp.com/nomad/docs/glossary#allocation) first become healthy. Use it when a [service check](https://developer.hashicorp.com/nomad/docs/job-specification/check) passes before the task can serve traffic.

{{< example file="nomad/ready-after.nomad.hcl" region="sablier" >}}

[Learn more about delaying readiness](/how-to-guides/readiness/ready-after/).

#### Ready on start {since="next"}

`"sablier.ready-on-start" = "true"` reports the task group ready as soon as Sablier sends the scale request, without waiting for its [allocations](https://developer.hashicorp.com/nomad/docs/glossary#allocation).

{{< example file="nomad/ready-on-start.nomad.hcl" region="sablier" >}}

[Learn more about marking ready on start](/how-to-guides/readiness/ready-on-start/).

#### Running hours {since="next"}

- `sablier.running-hours` keeps the task group running during a daily window.
- `sablier.running-days` limits that window to some weekdays.

{{< example file="nomad/running-hours.nomad.hcl" region="sablier" >}}

The window uses Sablier's local time. In the Sablier job, set the time zone with the `TZ` [environment variable](https://developer.hashicorp.com/nomad/docs/job-specification/env):

{{< example file="nomad/sablier.nomad.hcl" region="env" >}}

[Learn more about running hours](/how-to-guides/lifecycle/running-hours/).

#### Anti-affinity {since="next"}

`sablier.anti-affinity` lists groups the task group backs off from: Sablier keeps it idle while any of them has an active session.

{{< example file="nomad/anti-affinity.nomad.hcl" region="sablier" >}}

[Learn more about anti-affinity](/how-to-guides/anti-affinity/).

#### Scale instead of stop {since="next"}

Keep the task group running when idle instead of scaling it to zero:

- `sablier.idle.replicas` set to `1` or more is the [count](https://developer.hashicorp.com/nomad/docs/job-specification/group#count) when the session expires.
- `sablier.active.replicas` is the count on the next session, `1` by default.

{{< example file="nomad/scale-mode.nomad.hcl" region="sablier" >}}

Nomad supports the counts only. CPU and memory limits belong to the task [resources](https://developer.hashicorp.com/nomad/docs/job-specification/resources) block, so Sablier ignores `idle.cpu` and `idle.memory`, and their `active` counterparts.

[Learn more about scaling instead of stopping](/how-to-guides/scaling-resources/scale-mode/).

### Readiness

Sablier reports a task group:

- **stopped** when its [count](https://developer.hashicorp.com/nomad/docs/job-specification/group#count) is `0`.
- **ready** when as many [allocations](https://developer.hashicorp.com/nomad/docs/glossary#allocation) as the count are running and healthy.
- **starting** while an allocation is pending, or running but not healthy yet.
- **completed** when every allocation of a [batch job](https://developer.hashicorp.com/nomad/docs/concepts/scheduling/schedulers#batch) task group has completed.
- **error** when only failed or lost allocations are left, or when the job was stopped with `nomad job stop`.

An allocation is healthy when its [deployment](https://developer.hashicorp.com/nomad/docs/glossary#deployment) marks it healthy. With the default [`health_check`](https://developer.hashicorp.com/nomad/docs/job-specification/update#health_check), its tasks must run and its [service checks](https://developer.hashicorp.com/nomad/docs/job-specification/check) must pass for [`min_healthy_time`](https://developer.hashicorp.com/nomad/docs/job-specification/update#min_healthy_time). Give your services a check.

An allocation without deployment health, for example in a batch job, is ready as soon as it runs.

### Limitations

- One Sablier manages one Nomad [namespace](https://developer.hashicorp.com/nomad/docs/govern/namespaces).
- Sablier can't scale [system](https://developer.hashicorp.com/nomad/docs/concepts/scheduling/schedulers#system) and [sysbatch](https://developer.hashicorp.com/nomad/docs/concepts/scheduling/schedulers#system-batch) jobs: they have no count.
- Sablier skips [periodic](https://developer.hashicorp.com/nomad/docs/job-specification/periodic) and [parameterized](https://developer.hashicorp.com/nomad/docs/job-specification/parameterized) parent jobs, which never run themselves.
- A task group with a [scaling block](https://developer.hashicorp.com/nomad/docs/job-specification/scaling#min) must allow `min = 0`. Otherwise Nomad rejects the scale to zero.
- [CPU](/how-to-guides/scaling-resources/scale-cpu/) and [memory](/how-to-guides/scaling-resources/scale-memory/) throttling are not available on Nomad: Sablier only changes the count.
- [Block I/O throttling](/how-to-guides/scaling-resources/scale-blkio/) is not available on Nomad.

## Examples

- [`examples/nomad`](https://github.com/sablierapp/sablier/tree/main/examples/nomad): every job on this page, with a Nomad development agent and Sablier in Docker Compose, and instructions to run them.
