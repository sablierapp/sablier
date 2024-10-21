# Caddy Sablier Plugin

Caddy Sablier Plugin.

## Provider compatibility grid

| Provider                                | Dynamic | Blocking |
| --------------------------------------- | :-----: | :------: |
| [Docker](../providers/docker)             |    ✅    |    ✅     |
| [Docker Swarm](../providers/docker_swarm) |    ✅    |    ✅     |
| [Kubernetes](../providers/kubernetes)     |    ❌    |    ❌     |

## Install the plugin to Caddy

Because Caddy does not do runtime evaluation, you need to build the base image with the plugin source code.

In order to use the custom plugin for Caddy, you need to bundle it with Caddy.
Here I'll show you two options with Docker.

<!-- tabs:start -->

#### **Using the provided Dockerfile**

```bash
docker build https://github.com/sablierapp/sablier.git#v1.4.0-beta.3:plugins/caddy 
  --build-arg=CADDY_VERSION=2.6.4
  -t caddy:2.6.4-with-sablier
```

#### **Updating your Caddy Dockerfile**

```docker
ARG CADDY_VERSION=2.6.4
FROM caddy:${CADDY_VERSION}-builder AS builder

ADD https://github.com/sablierapp/sablier.git#v1.4.0-beta.3 /sablier

RUN xcaddy build \
    --with github.com/sablierapp/sablier/plugins/caddy=/sablier/plugins/caddy

FROM caddy:${CADDY_VERSION}

COPY --from=builder /usr/bin/caddy /usr/bin/caddy
```

<!-- tabs:end -->

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
