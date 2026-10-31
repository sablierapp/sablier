# Sablier Go client

A Go client for the [Sablier](https://sablierapp.dev) HTTP API, made for reverse-proxy plugins. It uses only the standard library and runs under [Yaegi](https://github.com/traefik/yaegi), the interpreter Traefik runs plugins with.

```sh
go get github.com/sablierapp/sablier/client
```

## Usage

Each call returns its typed result, the `*Response`, and an error. The `Response` is nil only when Sablier could not be reached. When Sablier answers with a status other than 2xx, the error is an `*ErrorResponse` and the `Response` is still returned, with its body readable, so a proxy can forward it as is.

```go
c, err := client.New("http://sablier:10000",
	client.WithUserAgent("my-plugin/1.0.0"))
if err != nil {
	return err
}

group := "my-app"
session, resp, err := c.StartBlocking(ctx, client.StartBlockingParams{Group: &group})
if resp == nil {
	return err // Sablier could not be reached
}
if errResp, ok := err.(*client.ErrorResponse); ok {
	// Sablier answered with an error, such as "group not found", that
	// errResp.Problem describes. resp.Body holds the answer to forward.
	return forward(errResp.Response)
}
if err != nil {
	return err // the body could not be read or decoded
}
// session.Session describes the instances; resp.Ready() is true.
```

The waiting page of the dynamic strategy comes back as the typed result:

```go
page, resp, err := c.StartDynamic(ctx, client.StartDynamicParams{Group: &group})
```

## Compatibility

### Sablier

The client works with any Sablier release from the one each call needs:

| Call | Endpoint | Sablier |
|---|---|---|
| `StartDynamic` | `GET /api/strategies/dynamic` | 1.0.0 |
| `StartBlocking` | `GET /api/strategies/blocking` | 1.0.0 |
| `StartPoke` | `GET /api/strategies/poke` | 1.17.0 |
| `Events` | `GET /api/events` | 1.13.0 |

Some parameters and fields need a later release:

| Parameter or field | Sablier |
|---|---|
| `Group` of the strategy calls | 1.4.0 |
| `ErrorResponse.Problem`, from problem detail responses | 1.8.3 |
| The `optional:` name prefix | 1.17.0 |
| `Names` and `Group` of `Events` | 1.19.0 |
| The session returned by `StartPoke` | 1.19.0 |

An older server ignores parameters it does not know, looks an `optional:` name up as written, and answers errors without a problem detail, so `Problem` is nil.

### Traefik

| Traefik | Yaegi | Status |
|---|---|---|
| 3.7 | v0.16.1 | Supported, tested |
| 2.11, 3.0 to 3.6 | v0.16.1 | Expected to work, not supported |
| 2.10 and older | v0.15.1 or older | Not supported |

### Supported versions

- **Sablier**: every release at or above the versions in the tables above. A client release that needs a newer server for a call says so in its changelog and in these tables.
- **Traefik**: the release lines [Traefik supports](https://doc.traefik.io/traefik/deprecation/releases/). CI runs the client under the Yaegi version of the latest Traefik release (see [`yaegitest`](yaegitest)). Older lines that ship the same Yaegi are expected to work, but they are not supported.
- **Go**: the client builds with Go 1.21 and later, the `go` directive in `go.mod`.

## Yaegi

Yaegi v0.16.1 runs less Go than the compiler. These rules keep the client inside what it supports:

| Not supported by Yaegi | Forbidden by |
|---|---|
| `errors.As` with an interpreted target type: it panics | `forbidigo` in [`.golangci.yml`](.golangci.yml) |
| The `min`, `max` and `clear` builtins | `forbidigo` |
| `unsafe` and `syscall`: Traefik only allows them with `useUnsafe` | `depguard` |
| Standard library API newer than Go 1.22 | `go vet` (`stdversion`), with the `go 1.21` directive |
| `range` over an int or a func | The `go 1.21` directive: they do not compile |
| `switch v := x(); v.Kind() {`: no case matches, silently | [`TestNoSwitchInitWithCallTag`](yaegi_test.go) |

Code that uses the client under Yaegi has the same limits. In particular, check for an `*ErrorResponse` with a type assertion, not `errors.As`. The client never wraps it. Yaegi also rejects method values of standard library types, such as `dialer.DialContext`; wrap them in a closure.
