> [!IMPORTANT]
> This plugin now has its own reepository: https://github.com/sablierapp/sablier-caddy-plugin
>
> This plugin will no longer receive updates.

# Caddy Sablier Plugin

- [Caddy Sablier Plugin](#caddy-sablier-plugin)
	- [Build the custom Caddy image with Sablier middleware in it](#build-the-custom-caddy-image-with-sablier-middleware-in-it)
		- [By using the provided Dockerfile](#by-using-the-provided-dockerfile)
		- [By updating your Caddy Dockerfile](#by-updating-your-caddy-dockerfile)
	- [Configuration](#configuration)
		- [Exemple with a minimal configuration](#exemple-with-a-minimal-configuration)
	- [Running end-to-end tests](#running-end-to-end-tests)

## Build the custom Caddy image with Sablier middleware in it

In order to use the custom plugin for Caddy, you need to bundle it with Caddy.
Here I'll show you two options with Docker.

### By using the provided Dockerfile

```bash
docker build https://github.com/sablierapp/sablier.git#v1.10.1:plugins/caddy -t caddy:with-sablier
```

**Note:** You can change `main` for any other branch (such as `beta`, or tags `v1.10.1`)

### By updating your Caddy Dockerfile

```Dockerfile
ARG CADDY_VERSION=2.9.1
FROM caddy:${CADDY_VERSION}-builder AS builder

RUN xcaddy build \
    --with github.com/sablierapp/sablier/plugins/caddy

FROM caddy:${CADDY_VERSION}

COPY --from=builder /usr/bin/caddy /usr/bin/caddy
```

## Configuration

You can have the following configuration:

```Caddyfile
:80 {
	route /my/route {
    sablier [<sablierURL>=http://sablier:10000] {
			[names container1,container2,...]
			[group mygroup]
			[session_duration 30m]
			dynamic {
				[display_name This is my display name]
				[show_details yes|true|on]
				[theme hacker-terminal]
				[refresh_frequency 2s]
			}
			blocking {
				[timeout 1m]
			}
		}
    reverse_proxy myservice:port
  }
}
```

### Exemple with a minimal configuration

Almost all options are optional and you can setup very simple rules to use the server default values.

```Caddyfile
:80 {
	route /my/route {
    sablier {
			group mygroup
			dynamic
		}
    reverse_proxy myservice:port
  }
}
```

## Running end-to-end tests

1. Build local sablier
  `docker build -t caddy:local .`
2. Build local caddy
  `docker build -t sablierapp/sablier:local ../..`
3. Run test
  `cd e2e/docker && bash ./run.sh`