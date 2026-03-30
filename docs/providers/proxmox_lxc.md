# Proxmox LXC

The Proxmox LXC provider communicates with the Proxmox VE API to start and stop LXC containers on demand.

## Use the Proxmox LXC provider

In order to use the Proxmox LXC provider you can configure the [provider.name](../configuration) property.

<!-- tabs:start -->

#### **File (YAML)**

```yaml
provider:
  name: proxmox_lxc
  proxmox-lxc:
    url: "https://proxmox.local:8006/api2/json"
    token-id: "root@pam!sablier"
    token-secret: "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
    tls-insecure: false
```

#### **CLI**

```bash
sablier start \
  --provider.name=proxmox_lxc \
  --provider.proxmox-lxc.url=https://proxmox.local:8006/api2/json \
  --provider.proxmox-lxc.token-id=root@pam!sablier \
  --provider.proxmox-lxc.token-secret=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
```

#### **Environment Variable**

```bash
SABLIER_PROVIDER_NAME=proxmox_lxc
SABLIER_PROVIDER_PROXMOX_LXC_URL=https://proxmox.local:8006/api2/json
SABLIER_PROVIDER_PROXMOX_LXC_TOKEN_ID=root@pam!sablier
SABLIER_PROVIDER_PROXMOX_LXC_TOKEN_SECRET=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
SABLIER_PROVIDER_PROXMOX_LXC_TLS_INSECURE=false
```

<!-- tabs:end -->

## Configuration

| Property | CLI Flag | Environment Variable | Default | Description |
|---|---|---|---|---|
| `url` | `--provider.proxmox-lxc.url` | `SABLIER_PROVIDER_PROXMOX_LXC_URL` | *(required)* | Proxmox VE API URL |
| `token-id` | `--provider.proxmox-lxc.token-id` | `SABLIER_PROVIDER_PROXMOX_LXC_TOKEN_ID` | *(required)* | API token ID (e.g. `root@pam!sablier`) |
| `token-secret` | `--provider.proxmox-lxc.token-secret` | `SABLIER_PROVIDER_PROXMOX_LXC_TOKEN_SECRET` | *(required)* | API token secret |
| `tls-insecure` | `--provider.proxmox-lxc.tls-insecure` | `SABLIER_PROVIDER_PROXMOX_LXC_TLS_INSECURE` | `false` | Skip TLS certificate verification (for self-signed certs) |

## Create a Proxmox API Token

1. In the Proxmox web UI, go to **Datacenter > Permissions > API Tokens**
2. Click **Add** and create a token for a user (e.g. `root@pam`)
3. Uncheck **Privilege Separation** so the token inherits the user's permissions
4. Note the **Token ID** (e.g. `root@pam!sablier`) and **Secret**

The token needs the following permissions on the LXC containers:
- `VM.PowerMgmt` — to start and stop containers
- `VM.Audit` — to read container status and configuration

## Register containers

For Sablier to work, it needs to know which LXC containers to start and stop.

You have to register your containers by opting-in with **Proxmox tags**.

```yaml
arch: amd64
cores: 2
hostname: whoami
memory: 4096
net0: name=eth0,bridge=vmbr0,hwaddr=BC:24:11:81:7C:C4,ip=dhcp,type=veth
ostype: debian
rootfs: local-lvm:vm-100-disk-0,size=8G
swap: 512
tags: sablier;sablier-group-mygroup
unprivileged: 1
```

### Add tags via the CLI

```bash
pct set 100 -tags "sablier;sablier-group-mygroup"
```

### Add tags via the Web UI

In the Proxmox web UI, select a container and click the **pencil icon** next to the tags in the toolbar (next to the container name) to edit tags.

### Tags reference

| Tag | Description |
|---|---|
| `sablier` | **Required.** Marks the container as managed by Sablier. |
| `sablier-group-<name>` | Optional. Assigns the container to a group. Defaults to `default` if not specified. |

## Instance naming

Sablier uses the LXC container **hostname** as the instance name. You can also reference containers by their **VMID** (e.g. `100`) or by **node/VMID** format (e.g. `pve1/100`).

!> Hostnames must be unique among Sablier-managed containers. If duplicate hostnames are detected, Sablier will return an error.

## Multi-node support

The Proxmox LXC provider automatically discovers all nodes in the cluster and scans for tagged containers across all of them. No additional configuration is required for multi-node setups.

## How does Sablier know when a container is ready?

Sablier checks the LXC container status reported by Proxmox. Additionally, for `running` containers, Sablier verifies that at least one non-loopback network interface has an IP address assigned before reporting the container as ready.

| Proxmox Status | Sablier Status |
|---|---|
| `running` (with IP) | Ready |
| `running` (no IP yet) | Not Ready |
| `stopped` | Not Ready |
| Other | Unrecoverable |
