#!/usr/bin/env bash
# Opts an existing container in, keeping the tags it already has.
# Run it as root on the node that hosts the container.
# Usage: ./enable.sh <vmid>
set -euo pipefail

vmid="$1"

# region enable
# pct set --tags replaces the whole list, so append to the current tags.
tags="$(pct config "$vmid" | awk '$1 == "tags:" {print $2}')"
pct set "$vmid" --tags "${tags:+$tags;}sablier"
# endregion
