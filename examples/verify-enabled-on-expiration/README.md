# `verify-enabled-on-expiration` Example

This example demonstrates `--provider.verify-enabled-on-expiration=true`.

The stack starts Sablier and one Docker container without `sablier.enable=true`.
The `test` target stops the container, requests it directly by `names` waits for the session to expire, then verifies the
container is still running. With expiration verification enabled, Sablier
inspects the instance before stopping it and skips the stop because the
instance is not labelled as Sablier-managed.

## Run

```bash
make test
```

Expected result:

```text
OK: expired unlabeled instance was inspected and left running
```

## Tear down

```bash
make down
```
