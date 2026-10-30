#!/bin/sh
# Run Sablier with podman run instead of Compose.
#
# Rootless Podman: point PODMAN_SOCKET at your user's socket first:
#
#   PODMAN_SOCKET=$XDG_RUNTIME_DIR/podman/podman.sock ./sablier.sh
set -eu

# region run
SABLIER_IMAGE=docker.io/sablierapp/sablier:1.20.0 # x-release-please-version

podman run --detach --name sablier \
  --volume "${PODMAN_SOCKET:-/run/podman/podman.sock}:/run/podman/podman.sock" \
  --security-opt label=disable \
  --publish 10000:10000 \
  "$SABLIER_IMAGE" \
  start --provider.name=podman
# endregion
