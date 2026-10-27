# Proxmox LXC Examples

Runnable files for the [Proxmox LXC provider](https://sablierapp.dev/providers/proxmox_lxc/).
The documentation includes these files directly, so what you read is what you
run. The shell scripts run as root on a [Proxmox VE](https://pve.proxmox.com/pve-docs/)
node; Sablier itself runs anywhere that reaches the Proxmox VE API on port 8006.

## Files

| File | Shows |
|---|---|
| `token.sh` | A role, a user and the API token Sablier authenticates with |
| `sablier.yaml` | Sablier's configuration for the Proxmox LXC provider |
| `compose.yml` | Sablier in Docker Compose, trusting the cluster CA |
| `whoami.sh` | A Debian container in the `whoami` group, the starting point for every other example |
| `enable.sh` | Opts an existing container in, keeping its other tags |
| `groups.sh` | Puts a container in two groups through the API, from any node |

Lines between `# region <name>` and `# endregion` are the excerpts the
documentation shows; the shell and YAML ignore them as comments.

## Run it

You need a Proxmox VE node with a `local` storage for templates, a `local-lvm`
storage for disks and a `vmbr0` bridge with DHCP. Override them with the
`TEMPLATE_STORAGE`, `ROOTFS_STORAGE` and `BRIDGE` environment variables of
`whoami.sh`.

On the node, create the token and the container:

```bash
./token.sh      # prints the token secret once: keep it
./whoami.sh     # creates container 200, stopped
```

On the machine that runs Sablier, copy the cluster CA, set `url` in
`sablier.yaml` to your node, and start Sablier:

```bash
scp root@pve1.example.com:/etc/pve/pve-root-ca.pem .
PROXMOX_TOKEN_SECRET=<token secret> docker compose up -d
```

Request the `whoami` group through the Sablier API, as a reverse proxy plugin
would. The request opens a one-minute session:

```bash
curl "http://localhost:10000/api/strategies/blocking?group=whoami&session_duration=1m"
```

Sablier starts container 200 and answers once it has an IP address
(`pct status 200` on the node shows `running`). One minute after the last
request, Sablier stops it. Run the same `curl` again to start it back.

Run `enable.sh` or `groups.sh` on your own containers to opt them in.
