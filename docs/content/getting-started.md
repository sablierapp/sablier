---
title: Getting Started
weight: 10
---

This guide will walk you through setting up Sablier as a scale-to-zero middleware with a reverse proxy.

![integration](/assets/img/integration.png)

## Identify Your Provider

The first thing you need to do is identify your [Provider](/providers/).

{{< callout type="info" >}}
A Provider is how Sablier interacts with your instances to scale them up and down to zero.
{{< /callout >}}

You can check the available providers [here](/providers/#available-providers).

## Identify Your Reverse Proxy

Once you've identified your [Provider](/providers/), you'll need to identify your [Reverse Proxy](/reverse-proxies/).

{{< callout type="info" >}}
Because Sablier is designed as an API that can be used independently, reverse proxy integrations act as clients of that API.
{{< /callout >}}

You can check the available reverse proxy plugins [here](/reverse-proxies/#available-reverse-proxies)

## Connect It All Together

- Let's say we're using the [Docker Provider](/providers/docker/).
- Let's say we're using the [Caddy Reverse Proxy Plugin](/reverse-proxies/caddy/).

{{% steps %}}

### Initial Setup with Caddy

Suppose this is your initial setup with Caddy. You have your reverse proxy with a Caddyfile that performs a simple reverse proxy on `/whoami`.

{{< tabs >}}
{{< tab name="docker-compose.yaml" >}}

```yaml
services:
  proxy:
    image: caddy:2.8.4
    ports:
      - "8080:80"
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile:ro

  whoami:
    image: acouvreur/whoami:v1.10.2
```

{{< /tab >}}
{{< tab name="Caddyfile" >}}

```Caddyfile
:80 {
	route /whoami {
		reverse_proxy whoami:80
	}
}
```

{{< /tab >}}
{{< /tabs >}}

Now you can run `docker compose up` and navigate to `http://localhost:8080/whoami` to see your service.

### Install Sablier with the Docker Provider

Add the Sablier container to the `docker-compose.yaml` file.

```yaml
services:
  proxy:
    image: caddy:2.8.4
    restart: unless-stopped
    ports:
      - "8080:80"
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile:ro

  whoami:
    image: acouvreur/whoami:v1.10.2
    restart: unless-stopped

  sablier:
    image: sablierapp/sablier:1.14.0 # x-release-please-version
    restart: always
    command:
        - start
        - --provider.name=docker
    volumes:
      - '/var/run/docker.sock:/var/run/docker.sock'
```

### Add the Sablier Caddy Plugin to Caddy

Because Caddy does not provide runtime plugin evaluation, we need to build Caddy with this specific plugin.

We'll use the provided Dockerfile to build the custom Caddy image.

```bash
docker build https://github.com/sablierapp/sablier-caddy-plugin.git 
  --build-arg=CADDY_VERSION=2.8.4
  -t caddy:2.8.4-with-sablier
```

Then change the image from `caddy:2.8.4` to `caddy:2.8.4-with-sablier`

```yaml
services:
  proxy:
    image: caddy:2.8.4-with-sablier
    restart: unless-stopped
    ports:
      - "8080:80"
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile:ro

  whoami:
    image: acouvreur/whoami:v1.10.2
    restart: unless-stopped

  sablier:
    image: sablierapp/sablier:1.14.0 # x-release-please-version
    restart: always
    command:
        - start
        - --provider.name=docker
    volumes:
      - '/var/run/docker.sock:/var/run/docker.sock'
```

### Configure Caddy to use the Sablier Caddy Plugin on the `whoami` service

This is how you opt in your services and link them with the plugin.

{{< tabs >}}
{{< tab name="docker-compose.yaml" >}}

```yaml
services:
  proxy:
    image: caddy:local
    restart: unless-stopped
    ports:
      - "8080:80"
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile:ro

  whoami:
    image: acouvreur/whoami:v1.10.2
    restart: unless-stopped
    labels:
      - sablier.enable=true
      - sablier.group=demo
  
  sablier:
    image: sablierapp/sablier:1.14.0 # x-release-please-version
    restart: always
    volumes:
      - '/var/run/docker.sock:/var/run/docker.sock'
```

{{< /tab >}}
{{< tab name="Caddyfile" >}}

```Caddyfile
:80 {
	route /whoami {
      sablier http://sablier:10000 {
        group demo
        session_duration 1m 
        dynamic {
            display_name My Whoami Service
        }
      }

	  reverse_proxy whoami:80
	}
}
```

Here we've configured the following for when accessing the service at `http://localhost:8080/whoami`:
- Containers with the label `sablier.group=demo` will be started on demand
- The period of inactivity after which containers should be shut down is one minute
- It uses the dynamic configuration and sets the display name to `My Whoami Service`

{{< /tab >}}
{{< /tabs >}}

{{< callout type="info" >}}
We've assigned the group `demo` to the service, which is how we identify the workload.
{{< /callout >}}

{{% /steps %}}
