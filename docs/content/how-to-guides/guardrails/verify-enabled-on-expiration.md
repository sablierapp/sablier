---
title: Verify enabled on expiration
weight: 166
---

Inspect an instance before stopping it when its session expires, and skip the stop if it is not labelled `sablier.enable=true`.

```yaml
# compose.yml
services:
  sablier:
    image: sablierapp/sablier:1.14.0 # x-release-please-version
    command:
      - start
      - --provider.name=docker
      - --provider.verify-enabled-on-expiration=true
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock

  unlabeled:
    image: acouvreur/whoami:v1.10.2
```

When the session for the unlabeled container expires, Sablier inspects it, finds no `sablier.enable=true` label, and leaves it running.

With `--provider.verify-enabled-on-expiration=true`, Sablier inspects an instance before stopping it when its session expires, and skips the stop if the instance is not labelled `sablier.enable=true`. This protects containers that are no longer (or were never) Sablier-managed from being stopped when an old session expires.

## When to use it

Use this when a session may outlive a container's Sablier labels, for example if a container is redeployed without the label, and you want to be sure Sablier never stops something it does not currently manage.

## Flags

- [`--provider.verify-enabled-on-expiration`](/reference/cli/): inspect an instance on session expiration and only stop it if it is still labelled `sablier.enable=true`.

See the [runnable example](https://github.com/sablierapp/sablier/tree/main/examples/verify-enabled-on-expiration).
