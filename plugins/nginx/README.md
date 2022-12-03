# Nginx NJS Sablier Module

Nginx NJS Sablier Module

**Kubernetes is not supported yet**

## Configuration

1. Load the `ngx_http_js_module.so` in the main nginx config file `/etc/nginx/nginx.conf`
    ```
    load_module modules/ngx_http_js_module.so;
    ```
2. Copy/volume the `sablier.js` file to `/etc/nginx/conf.d/sablier.js`
3. Use this sample for your APIs
    ```nginx
    js_import conf.d/sablier.js;

    resolver 127.0.0.11 valid=10s ipv6=off;

    server {
    listen 80;

        subrequest_output_buffer_size 32k;

        # The internal location to reach sablier API
        set $sablierUrl /sablier;
        # Shared variable for default session duration
        set $sablierSessionDuration 1m;

        # internal location for sablier middleware
        # here, the sablier API is a container named "sablier" inside the same network as nginx
        location /sablier/ {
            internal;
            proxy_method GET;
            proxy_pass http://sablier:10000/;
        }

        # A named location that can be used by the sablier middleware to redirect
        location @whoami {
            # Here is your container name, same in 
            #             set $sablierNames whoami;
            # Use variable in order to refresh DNS cache
            set $whoami_server whoami;
            proxy_pass http://$whoami_server:80;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        }

        # The actual location to match your API
        # Will answer by a waiting page or redirect to your app if
        # it is already up
        location /dynamic/whoami {
            set $sablierDynamicShowDetails true;
            set $sablierDynamicRefreshFrequency 5s;
            set $sablierNginxInternalRedirect @whoami;
            # Here is your container name
            set $sablierNames whoami;
            set $sablierDynamicName "Dynamic Whoami";
            set $sablierDynamicTheme hacker-terminal;
            js_content sablier.call;
        }
    }
    ```

### Available variables

You can configure the middleware behavior with the following variables:

**General Configuration**

- `set $sablierUrl` The internal routing to reach Sablier API
- `set $sablierNames` Comma separated names of containers/services/deployments etc.
- `set $sablierSessionDuration` The session duration after which containers/services/deployments instances are shutdown
- `set $sablierNginxInternalRedirect` The internal location for the service to redirect e.g. @nginx

**Dynamic Configuration**

*if any of these variables is set, then all Blocking Configuration is ignored*

- `set $sablierDynamicName`
- `set $sablierDynamicShowDetails` Set to true or false to show details specifcally for this middleware, unset to use Sablier server defaults
- `set $sablierDynamicTheme` The theme to use
- `set $sablierDynamicRefreshFrequency` The loading page refresh frequency

**Blocking Configuration**

- `set $sablierBlockingTimeout` waits until services are up and running but will not wait more than `timeout`

## Development

Change the `njs/sablier.js` configuration and start the tests for the given provider `e2e/<provider>.sh` (docker, kubernetes, etc.)