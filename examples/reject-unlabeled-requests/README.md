# `reject-unlabeled-requests` Example

This example demonstrates `--provider.reject-unlabeled-requests=true`.

The stack starts Sablier and one Docker container without `sablier.enable=true`.
The `test` target sends a direct `names` request for that container. Because the
target is not labelled as Sablier-managed, the request is rejected.

## Run

```bash
make test
```

Expected result:

```text
OK: request was rejected
```

## Tear down

```bash
make down
```
