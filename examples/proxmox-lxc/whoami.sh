#!/usr/bin/env bash
# Creates "whoami", a Debian container that Sablier starts on demand.
# Run it as root on a Proxmox VE node. Override the defaults with environment
# variables, for example: VMID=300 BRIDGE=vmbr1 ./whoami.sh
set -euo pipefail

VMID="${VMID:-200}"
TEMPLATE_STORAGE="${TEMPLATE_STORAGE:-local}"
ROOTFS_STORAGE="${ROOTFS_STORAGE:-local-lvm}"
BRIDGE="${BRIDGE:-vmbr0}"

# Download the latest Debian system template, unless it is already there.
pveam update
template="$(pveam available --section system | awk '$2 ~ /^debian-[0-9]+-standard_/ {print $2}' | sort -V | tail -n 1)"
if ! pveam list "$TEMPLATE_STORAGE" | grep -qF "$template"; then
  pveam download "$TEMPLATE_STORAGE" "$template"
fi

# The container is created stopped: Sablier starts it on the first request.
# region container
pct create "$VMID" "$TEMPLATE_STORAGE:vztmpl/$template" \
  --hostname whoami \
  --tags "sablier;sablier-group-whoami" \
  --net0 "name=eth0,bridge=$BRIDGE,ip=dhcp" \
  --rootfs "$ROOTFS_STORAGE:4" \
  --unprivileged 1
# endregion
