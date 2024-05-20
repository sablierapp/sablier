# Apache APISIX

## Provider compatibility grid

| Provider                                | Dynamic |   Blocking    |
|-----------------------------------------|:-------:|:-------------:|
| [Docker](/providers/docker)             |    ✅    |       ✅       |
| [Docker Swarm](/providers/docker_swarm) |    ✅    |       ✅       |
| [Kubernetes](/providers/kubernetes)     |    ❓    |       ❓       |

## Install the plugin to Apache APISIX

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
