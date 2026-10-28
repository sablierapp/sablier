---
title: Proxmox LXC
weight: 5
since: v1.12.0
aliases:
  - /tutorials/providers/proxmox_lxc/
---

## Introduction

The Proxmox LXC provider starts and stops [LXC containers](https://pve.proxmox.com/pve-docs/chapter-pct.html) on a [Proxmox VE](https://pve.proxmox.com/pve-docs/) node or [cluster](https://pve.proxmox.com/pve-docs/chapter-pvecm.html). You opt a container in with [tags](https://pve.proxmox.com/pve-docs/chapter-pve-gui.html#gui_tags). Sablier runs outside Proxmox VE and calls the [Proxmox VE API](https://pve.proxmox.com/wiki/Proxmox_VE_API) with an [API token](https://pve.proxmox.com/pve-docs/chapter-pveum.html#pveum_tokens).

## Install Sablier

Sablier needs an [API token](https://pve.proxmox.com/pve-docs/chapter-pveum.html#pveum_tokens) with two [privileges](https://pve.proxmox.com/pve-docs/chapter-pveum.html#_privileges) on the containers it manages:

- **`VM.Audit`** to list the containers and read their status, configuration and IP addresses.
- **`VM.PowerMgmt`** to start and stop them.

Create a [role](https://pve.proxmox.com/pve-docs/chapter-pveum.html#pveum_roles) with them, a dedicated [user](https://pve.proxmox.com/pve-docs/chapter-pveum.html#pveum_users) and its token. Run [`pveum`](https://pve.proxmox.com/pve-docs/pveum.1.html) as root on any node:

{{< example file="proxmox-lxc/token.sh" region="token" >}}

`pveum user token add` prints the token secret once: store it. Sablier only sees the containers where the token has `VM.Audit`. To limit it to some containers, grant the role on `/vms/<vmid>` or on a pool instead of `/vms`; see [objects and paths](https://pve.proxmox.com/pve-docs/chapter-pveum.html#_objects_and_paths).

Then configure Sablier with the API URL of any node and the token ID:

{{< example file="proxmox-lxc/sablier.yaml" region="provider" >}}

Pass the secret in the `SABLIER_PROVIDER_PROXMOX_LXC_TOKEN_SECRET` environment variable, so it stays out of the file.

{{< tabs >}}
{{< tab name="Docker Compose" >}}

Run Sablier on any host that reaches the API on port 8006:

{{< example file="proxmox-lxc/compose.yml" >}}

The image contains no CA certificates. By default, each node serves a certificate signed by the [cluster CA](https://pve.proxmox.com/pve-docs/chapter-sysadmin.html#_certificates_for_intra_cluster_communication), so copy `/etc/pve/pve-root-ca.pem` from a node next to `compose.yml`:

```bash
scp root@pve1.example.com:/etc/pve/pve-root-ca.pem .
PROXMOX_TOKEN_SECRET=<token secret> docker compose up -d
```

[`SSL_CERT_FILE`](https://pkg.go.dev/crypto/x509#SystemCertPool) makes Sablier trust that file. If your nodes use [another certificate](https://pve.proxmox.com/pve-docs/chapter-sysadmin.html#sysadmin_certificate_management), mount the CA that signed it instead.

{{< /tab >}}
{{< tab name="Binary" >}}

Download the binary from the [releases](https://github.com/sablierapp/sablier/releases) page. Run it on any host that reaches the API on port 8006:

```bash
export SABLIER_PROVIDER_PROXMOX_LXC_TOKEN_SECRET=<token secret>
sablier start --configFile sablier.yaml
```

If the host doesn't trust the [cluster CA](https://pve.proxmox.com/pve-docs/chapter-sysadmin.html#_certificates_for_intra_cluster_communication), copy `/etc/pve/pve-root-ca.pem` from a node and point [`SSL_CERT_FILE`](https://pkg.go.dev/crypto/x509#SystemCertPool) at it.

{{< /tab >}}
{{< /tabs >}}

Sablier calls the API at startup and exits if it can't connect. Reverse proxy plugins reach Sablier at `http://<sablier host>:10000`, or at `http://sablier:10000` from the same Compose project.

## Usage example

Opt a [container](https://pve.proxmox.com/pve-docs/chapter-pct.html) in with the `sablier` [tag](https://pve.proxmox.com/pve-docs/chapter-pve-gui.html#gui_tags), and give it a group with a `sablier-group-<name>` tag. This [`pct create`](https://pve.proxmox.com/pve-docs/pct.1.html) creates the `whoami` container, stopped:

{{< example file="proxmox-lxc/whoami.sh" region="container" >}}

Then configure your [reverse proxy](/reverse-proxies/) to call Sablier for the `whoami` group in front of the container's address. On the first request, Sablier starts the container and holds the request, or shows a waiting page, until the container has an IP address. When the session expires, Sablier stops it.

{{< callout type="info" >}}
Sablier stops opted-in containers that are running when it starts. A container you start afterwards keeps running until its first session expires, unless you [stop externally-started instances](/features/stop-external-starts/).
{{< /callout >}}

## Provider reference

### Configuration

| Option | Default | Description |
|---|---|---|
| [`--provider.proxmox-lxc.url`](/reference/cli/#opt-provider-proxmox-lxc-url) | | Required. The API URL of any node, such as `https://pve1.example.com:8006/api2/json`. |
| [`--provider.proxmox-lxc.token-id`](/reference/cli/#opt-provider-proxmox-lxc-token-id) | | Required. The API token ID, in the form `user@realm!token`. |
| [`--provider.proxmox-lxc.token-secret`](/reference/cli/#opt-provider-proxmox-lxc-token-secret) | | Required. The API token secret. |
| [`--provider.proxmox-lxc.tls-insecure`](/reference/cli/#opt-provider-proxmox-lxc-tls-insecure) | `false` | Skip TLS certificate verification. Prefer trusting the cluster CA. |

Every `--provider.*` option that is not provider-specific also applies; see the [CLI reference](/reference/cli/). With [tracing](/features/tracing/) on, each Proxmox VE API call is a child span, with or without `tls-insecure`.

### Workloads

Sablier manages [LXC containers](https://pve.proxmox.com/pve-docs/chapter-pct.html) on every node of the [cluster](https://pve.proxmox.com/pve-docs/chapter-pvecm.html):

- **Start** calls the API's `status/start`, like [`pct start`](https://pve.proxmox.com/pve-docs/pct.1.html), and follows the start task until it ends.
- **Stop** calls the API's `status/stop`, like [`pct stop`](https://pve.proxmox.com/pve-docs/pct.1.html), and waits for the task.

{{< callout type="warning" >}}
A stop is abrupt: Proxmox VE kills every process in the container, without the clean shutdown of `pct shutdown`. Make sure your services tolerate it.
{{< /callout >}}

A cluster needs no extra setting. Sablier lists the nodes through the node in `url`, and [`pveproxy`](https://pve.proxmox.com/pve-docs/pveproxy.8.html) forwards each call to the node that hosts the container. If a node doesn't answer, Sablier logs a warning and skips its containers.

### Instance names

A reverse proxy can target one container by name instead of a group. The container of the usage example is named:

```text
whoami
```

The name is the container's hostname, set with `--hostname`.

- **Keep hostnames unique across the cluster.** If two tagged containers share a hostname, Sablier logs a warning and ignores that hostname until you rename one; request them by ID meanwhile. A tagged container always wins over an untagged one with the same hostname.
- **The [CT ID](https://pve.proxmox.com/pve-docs/chapter-pct.html#pct_general) works too.** A request for `200` or `pve1/200` resolves to the container's hostname and shares its session.

Prefer groups: a name ties your proxy configuration to one container.

### Labels

Proxmox VE has no key/value labels, so Sablier reads the container's [tags](https://pve.proxmox.com/pve-docs/chapter-pve-gui.html#gui_tags):

{{< example file="proxmox-lxc/groups.sh" region="groups" >}}

- **`sablier` opts the container in.** It stands for `sablier.enable=true`.
- **`sablier-group-<name>` adds the container to a group.** It stands for `sablier.group`, with one tag per group.
- **Tags must match exactly.** Sablier ignores `Sablier`, `sablier=true` or `sablier.group=whoami`.
- **No other setting has a tag.** See the [limitations](#limitations).

Set tags in the web interface with the pencil icon next to the container's tags, or with [`pct set`](https://pve.proxmox.com/pve-docs/pct.1.html) or [`pvesh`](https://pve.proxmox.com/pve-docs/pvesh.1.html). Sablier rereads them every 30 seconds, without a restart.

### Workload settings

Each excerpt below comes from a script in [`examples/proxmox-lxc`](https://github.com/sablierapp/sablier/tree/main/examples/proxmox-lxc).

#### Enable {since="v1.12.0"}

The `sablier` tag opts the container in. It is required. See the [usage example](#usage-example) for a new container. For an existing one, append the tag to its current tags:

{{< example file="proxmox-lxc/enable.sh" region="enable" >}}

`--tags` replaces the whole tag list, so keep the tags the container already has.

#### Group {since="v1.12.0"}

A `sablier-group-<name>` tag puts the container in the `<name>` group. A session for any of its groups starts it. Without one, the container is in the `default` group.

Add one tag per group, since v1.13.0. Earlier releases only read the first one.

{{< example file="proxmox-lxc/groups.sh" region="groups" >}}

Remove a group tag, or the `sablier` tag, and Sablier drops the container from that group at its next scan.

[Learn more about groups](/features/groups/).

### Readiness

Sablier reports a container:

- **ready** when Proxmox VE reports it `running` and one of its interfaces other than `lo` has an IPv4 or IPv6 address.
- **starting** while the start task runs, or while the container runs without an IP address.
- **stopped** when Proxmox VE reports it `stopped`.
- **error** for 30 seconds after a failed start task, with the task's exit status. On the next check after that, Sablier starts the container again.

Give each container a [network interface](https://pve.proxmox.com/pve-docs/chapter-pct.html#pct_container_network) that gets an address: without one, it never reports ready. If Sablier can't read the interfaces, it reports the container ready.

Sablier doesn't check the service inside the container. The waiting page can clear before your application accepts connections.

### Limitations

- Sablier manages LXC containers only, not [virtual machines](https://pve.proxmox.com/pve-docs/chapter-qm.html).
- Tags can't carry values, so these settings are not available: [Ready after](/features/ready-after/), [Ready on start](/features/ready-on-start/), [Running hours](/features/running-hours/), [Anti-affinity](/features/anti-affinity/), [Scale instead of stop](/features/scale-mode/), and [CPU](/features/cpu-limits/), [memory](/features/memory-limits/) or [block I/O](/features/disk-io-limits/) throttling.
- Proxmox VE has no event stream, so Sablier polls the API every 10 seconds. It notices a container started or stopped outside Sablier within that delay, and [stops an externally-started one](/features/stop-external-starts/) when it notices it.

## Examples

- [`examples/proxmox-lxc`](https://github.com/sablierapp/sablier/tree/main/examples/proxmox-lxc): every script and file on this page, with instructions to run them.
