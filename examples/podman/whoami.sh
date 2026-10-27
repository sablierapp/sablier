#!/bin/sh
# Create the whoami container of compose.yml with podman run.
set -eu

# region run
podman create --name whoami \
  --label sablier.enable=true \
  --label sablier.group=whoami \
  --health-cmd '["/mimic", "healthcheck"]' \
  --health-interval 5s \
  --publish 8080:80 \
  docker.io/sablierapp/mimic:v0.3.3 -healthy=true
# endregion
