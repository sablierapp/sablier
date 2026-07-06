---
title: Reject unlabeled requests
weight: 164
---

Only serve requests for containers that carry the `sablier.enable=true` label, and reject requests targeting anything else.

```yaml
# compose.yml
services:
  sablier:
    image: sablierapp/sablier:1.14.0 # x-release-please-version
    command:
      - start
      - --provider.name=docker
      - --provider.reject-unlabeled-requests=true
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock

  unmanaged:
    image: acouvreur/whoami:v1.10.2
```

A direct request for the unlabeled `unmanaged` container is rejected because it is not labelled as Sablier-managed.

By default Sablier will act on any instance name it receives. With `--provider.reject-unlabeled-requests=true`, Sablier only serves requests for containers that carry the `sablier.enable=true` label and rejects requests targeting anything else.

## When to use it

Use this to make sure only explicitly opted-in containers can be managed through Sablier, so a request for an unlabeled container is refused instead of acted upon.

## Flags

- [`--provider.reject-unlabeled-requests`](/reference/cli/): reject requests targeting containers that are not labelled `sablier.enable=true`.

See the [runnable example](https://github.com/sablierapp/sablier/tree/main/examples/reject-unlabeled-requests).
