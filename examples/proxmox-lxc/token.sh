#!/usr/bin/env bash
# Creates the API token Sablier uses to reach the Proxmox VE API.
# Run it as root on any node of the cluster. The last command prints the
# token secret, once: store it for SABLIER_PROVIDER_PROXMOX_LXC_TOKEN_SECRET.
set -euo pipefail

# region token
# A role with the only two privileges Sablier needs.
pveum role add Sablier --privs "VM.Audit VM.PowerMgmt"

# A dedicated user with that role on every guest.
pveum user add sablier@pve --comment "Sablier"
pveum acl modify /vms --users sablier@pve --roles Sablier

# The token sablier@pve!sablier, with the same privileges as its user.
pveum user token add sablier@pve sablier --privsep 0
# endregion
