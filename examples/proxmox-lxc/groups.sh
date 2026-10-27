#!/usr/bin/env bash
# Puts an existing container in the "whoami" and "tools" groups.
# Run it as root on any node of the cluster: pvesh forwards the call to the
# node that hosts the container.
# Usage: ./groups.sh <node> <vmid>
set -euo pipefail

node="$1"
vmid="$2"

# region groups
pvesh set "/nodes/$node/lxc/$vmid/config" --tags "sablier;sablier-group-whoami;sablier-group-tools"
# endregion
