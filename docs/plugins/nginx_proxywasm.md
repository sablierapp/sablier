# Nginx Plugin

The Nginx Plugin is a WASM Plugin written with the Proxy Wasm SDK.

## Provider compatibility grid

| Provider                                | Dynamic | Blocking |
|-----------------------------------------|:-------:|:--------:|
| [Docker](../providers/docker)             |    ✅    |    ✅     |
| [Docker Swarm](../providers/docker_swarm) |    ❓    |    ❓     |
| [Kubernetes](../providers/kubernetes)     |    ❓    |    ❓     |

# Install ngx_wasm_module

Install https://github.com/Kong/ngx_wasm_module with version 0.6.0 at least.

Example for a Dockerfile:

```dockerfile
FROM ubuntu:25.04

RUN apt update && apt install -y libatomic1

ADD https://github.com/Kong/ngx_wasm_module/releases/download/prerelease-0.6.0/wasmx-prerelease-0.6.0-v8-x86_64-ubuntu22.04.tar.gz wasmx.tar.gz

RUN mkdir /etc/nginx
RUN tar -xvf wasmx.tar.gz
RUN mv /wasmx-prerelease-0.6.0-v8-x86_64-ubuntu22.04/* /etc/nginx/

WORKDIR /etc/nginx

CMD [ "./nginx", "-g", "daemon off;" ]
```

## Configuration

```nginx
# nginx.conf
events {}

# nginx master process gets a default 'main' VM
# a new top-level configuration block receives all configuration for this main VM
wasm {
    module proxywasm_sablier_plugin /wasm/sablierproxywasm.wasm;
}

http {
    access_log /dev/stdout;

    # internal docker resolver, see /etc/resolv.conf on proxy container
    # needed for docker name resolution
    resolver 127.0.0.11 valid=1s ipv6=off;

    server {
        listen 8080;

        location /dynamic {
            proxy_wasm  proxywasm_sablier_plugin '{ "sablier_url": "sablier:10000", "names": ["docker_classic_e2e-whoami-1"], "session_duration": "1m", "dynamic": { "display_name": "Dynamic Whoami", "theme": "hacker-terminal" } }';

            # force dns resolution by using a variable 
            # because container will be restarted and change ip a lot of times
            set $proxy_pass_host whoami:80$request_uri;
            proxy_pass  http://$proxy_pass_host;
            proxy_set_header Host localhost:8080; # e2e test compliance
        }

        location /blocking {
            wasm_socket_read_timeout 60s; # Blocking hangs the request
            proxy_wasm  proxywasm_sablier_plugin '{ "sablier_url": "sablier:10000", "names": ["docker_classic_e2e-whoami-1"], "session_duration": "1m", "blocking": { "timeout": "30s" } }';
    
            # force dns resolution by using a variable 
            # because container will be restarted and change ip a lot of times
            set $proxy_pass_host whoami:80$request_uri;
            proxy_pass  http://$proxy_pass_host;
            proxy_set_header Host localhost:8080; # e2e test compliance
        }
    }
}
```