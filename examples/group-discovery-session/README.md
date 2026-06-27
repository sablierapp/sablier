# `group-discovery-session` Example

This example demonstrates `--sessions.create-on-group-discovery=true`.

The stack starts one labelled Docker container and Sablier. Sablier discovers
the `whoami` group and creates a default-duration session immediately, without
waiting for a first request.

## Run

```bash
make test
```

Expected result:

```text
OK: discovered group session expired and stopped sablier-group-discovery-whoami
```

## Tear down

```bash
make down
```
