# Troubleshooting Guide

This guide helps you debug and resolve common issues with Sablier. If you don't find your issue here, please check our [Discord](https://discord.gg/WXYp59KeK9) or [open an issue](https://github.com/sablierapp/sablier/issues).

## Table of Contents

- [Troubleshooting Guide](#troubleshooting-guide)
  - [Table of Contents](#table-of-contents)
  - [General Debugging](#general-debugging)
    - [Enable Debug Logging](#enable-debug-logging)
    - [Check Sablier Health](#check-sablier-health)
    - [Verify Configuration](#verify-configuration)
  - [Common Issues](#common-issues)
    - [Blank Page / Theme Not Loading](#blank-page--theme-not-loading)
    - [Service Not Started](#service-not-started)
    - [Group Not Found](#group-not-found)
    - [Instance Does Not Exist](#instance-does-not-exist)
    - [Container Exited with Error Code](#container-exited-with-error-code)
    - [Timeout Errors](#timeout-errors)
    - [Connection Issues](#connection-issues)
  - [Provider-Specific Issues](#provider-specific-issues)
    - [Docker](#docker)
    - [Docker Swarm](#docker-swarm)
    - [Kubernetes](#kubernetes)
    - [Podman](#podman)
  - [Reverse Proxy Issues](#reverse-proxy-issues)
    - [Traefik](#traefik)
    - [Caddy](#caddy)
    - [Nginx](#nginx)
    - [Istio / Envoy](#istio--envoy)
    - [Apache APISIX](#apache-apisix)
  - [Configuration Issues](#configuration-issues)
    - [Configuration Not Loaded](#configuration-not-loaded)
    - [Environment Variables Not Working](#environment-variables-not-working)
    - [Invalid Log Level](#invalid-log-level)
  - [Performance Issues](#performance-issues)
    - [High CPU Usage](#high-cpu-usage)
    - [Memory Leaks](#memory-leaks)
  - [Getting More Help](#getting-more-help)
    - [Before Asking for Help](#before-asking-for-help)
    - [Support Channels](#support-channels)
    - [Plugin-Specific Issues](#plugin-specific-issues)

---

## General Debugging

### Enable Debug Logging

Enable debug logging to get detailed information about Sablier's operations:

**Using configuration file:**
```yaml
logging:
  level: debug
```

**Using environment variable:**
```bash
LOGGING_LEVEL=debug
```

**Using command-line argument:**
```bash
sablier start --logging.level=debug
```

Available log levels (from most to least verbose):
- `debug` - Detailed debugging information
- `info` - General informational messages (default)
- `warn` - Warning messages
- `error` - Error messages only

### Check Sablier Health

Use the health endpoint to verify Sablier is running:

**HTTP Request:**
```bash
curl http://localhost:10000/health
```

**Using the CLI:**
```bash
sablier health --url http://localhost:10000/health
```

**Expected Responses:**
- `200 OK` - Sablier is healthy and ready
- `503 Service Unavailable` - Sablier is terminating

**Docker Compose Healthcheck:**
```yaml
services:
  sablier:
    image: sablierapp/sablier:1.10.1
    healthcheck:
      test: ["CMD", "sablier", "health"]
      interval: 30s
      timeout: 10s
      retries: 3
```

### Verify Configuration

Check which configuration Sablier is using:

1. **Configuration file locations** (checked in order):
   - `/etc/sablier/sablier.yaml`
   - `$XDG_CONFIG_HOME/sablier.yaml`
   - `$HOME/.config/sablier.yaml`
   - `./sablier.yaml` (current directory)

2. **Override with custom config file:**
   ```bash
   sablier start --configFile=/path/to/config.yaml
   ```

3. **Configuration precedence** (highest to lowest):
   - Command-line arguments
   - Environment variables
   - Configuration file
   - Default values

---

## Common Issues

### Blank Page / Theme Not Loading

**Symptoms:**
- Browser shows a blank white page
- Theme displays incorrectly
- Error: "Theme not found"

**Possible Causes & Solutions:**

1. **Theme does not exist**

   **Error message:**
   ```json
   {
     "type": "https://sablierapp.dev/#/errors?id=theme-not-found",
     "title": "Theme not found",
     "status": 404,
     "detail": "The theme you requested does not exist among the default themes and the custom themes (if any).",
     "requestTheme": "my-custom-theme",
     "availableTheme": ["hacker-terminal", "ghost", "matrix", "shuffle"]
   }
   ```

   **Solution:**
   - Check the theme name in your configuration
   - Use one of the default themes: `hacker-terminal`, `ghost`, `matrix`, `shuffle`
   - If using a custom theme, verify it exists in your custom themes folder

2. **Custom themes path not configured correctly**

   **Check configuration:**
   ```yaml
   strategy:
     dynamic:
       custom-themes-path: /path/to/themes
   ```

   **Verify:**
   - Path is absolute
   - Directory exists and is readable
   - Theme files have `.html` extension
   - Files are valid Go templates

3. **Incorrect default theme**

   **Configuration:**
   ```yaml
   strategy:
     dynamic:
       default-theme: hacker-terminal  # Must be exact name
   ```

**Debug Steps:**
1. Enable debug logging
2. Check Sablier logs for theme loading messages
3. Verify theme files are in the correct location
4. Test with a default theme first

---

### Service Not Started

**Symptoms:**
- Waiting page shows but service never starts
- Instance status shows error
- "Instance does not exist" error

**Possible Causes & Solutions:**

1. **Workload does not exist**

   **Verify workload exists:**
   ```bash
   # Docker
   docker ps -a --filter "name=myservice"
   
   # Docker Swarm
   docker service ls
   
   # Kubernetes
   kubectl get deployment myservice
   
   # Podman
   podman ps -a --filter "name=myservice"
   ```

   **Solution:**
   - Create the workload first
   - Verify the name matches exactly (case-sensitive)
   - Check namespace (for Kubernetes)

2. **Incorrect labels/annotations**

   **Docker/Podman:**
   ```yaml
   services:
     myservice:
       labels:
         - "sablier.enable=true"
         - "sablier.group=mygroup"
   ```

   **Kubernetes:**
   ```yaml
   metadata:
     labels:
       sablier.enable: "true"
       sablier.group: "mygroup"
   ```

3. **Provider connection issues**

   **Check provider configuration:**
   ```yaml
   provider:
     name: docker  # docker, swarm, kubernetes, podman
   ```

   **Verify connectivity:**
   - Docker: Can Sablier access `/var/run/docker.sock`?
   - Kubernetes: Is kubeconfig valid?
   - Podman: Can Sablier access the Podman socket?

---

### Group Not Found

**Symptoms:**
- Error: "group not found"
- HTTP 404 response
- Waiting page doesn't load

**Error Response:**
```json
{
  "type": "https://sablierapp.dev/#/errors?id=group-not-found",
  "title": "Group not found",
  "status": 404,
  "detail": "group mygroup not found",
  "group": "mygroup",
  "availableGroups": ["group1", "group2", "group3"]
}
```

**Solution:**

1. **Check group name:**
   - Group names are case-sensitive
   - Check for typos in labels/annotations

2. **Verify labels on workload:**
   ```bash
   # Docker
   docker inspect mycontainer | grep sablier.group
   
   # Kubernetes
   kubectl get deployment mydeployment -o yaml | grep sablier.group
   ```

3. **List available groups:**
   - Check the `availableGroups` in the error response
   - Ensure your workload has the correct `sablier.group` label

---

### Instance Does Not Exist

**Symptoms:**
- Error: "instance does not exist"
- Status shows as error in theme

**Debug Steps:**

1. **Check if container/pod exists:**
   ```bash
   # Docker
   docker ps -a --filter "label=sablier.group=mygroup"
   
   # Kubernetes
   kubectl get pods -l sablier.group=mygroup --all-namespaces
   ```

2. **Verify Sablier has permissions:**
   - Docker: Volume mount for Docker socket
   - Kubernetes: ServiceAccount with proper RBAC
   - Podman: Socket permissions

3. **Check provider logs:**
   ```bash
   # Enable debug logging
   sablier start --logging.level=debug
   ```

---

### Container Exited with Error Code

**Symptoms:**
- Instance status: "Unrecoverable"
- Message: "container exited with code 'X'"
- Container starts but immediately stops

**Error Example:**
```
Status: Unrecoverable
Message: container exited with code "137"
```

**Common Exit Codes:**
- `137` - Container killed (OOM or manual kill)
- `139` - Segmentation fault
- `143` - Graceful termination (SIGTERM)
- `1` - Application error

**Solutions:**

1. **Check container logs:**
   ```bash
   docker logs <container-id>
   ```

2. **Verify resource limits:**
   - Increase memory limits if OOM (exit code 137)
   - Check CPU constraints

3. **Fix application errors:**
   - Review application logs
   - Fix startup failures
   - Ensure all dependencies are available

4. **Check for configuration issues:**
   - Environment variables
   - Volume mounts
   - Network settings

---

### Timeout Errors

**Symptoms:**
- Error: "timeout after X duration"
- Requests hang then fail
- Blocking strategy times out

**Error Response:**
```json
{
  "type": "https://sablierapp.dev/#/errors?id=timeout",
  "title": "Timeout",
  "status": 504,
  "detail": "timeout after 1m0s"
}
```

**Solutions:**

1. **Increase blocking timeout:**
   ```yaml
   strategy:
     blocking:
       default-timeout: 5m  # Increase from default 1m
   ```

2. **Check workload startup time:**
   - Some applications take longer to start
   - Increase timeout to match reality
   - Optimize container startup (use smaller images, faster initialization)

3. **Verify health checks:**
   - Ensure application is actually ready
   - Check readiness probes (Kubernetes)
   - Verify service is listening on correct port

---

### Connection Issues

**Symptoms:**
- Cannot connect to Sablier API
- Reverse proxy cannot reach Sablier
- "connection refused" errors

**Debug Steps:**

1. **Verify Sablier is running:**
   ```bash
   curl http://localhost:10000/health
   ```

2. **Check port configuration:**
   ```yaml
   server:
     port: 10000  # Default port
   ```

3. **Verify network connectivity:**
   ```bash
   # From reverse proxy container
   ping sablier
   curl http://sablier:10000/health
   ```

4. **Check Docker network:**
   ```bash
   docker network ls
   docker network inspect <network-name>
   ```

5. **Firewall rules:**
   - Ensure port 10000 (or configured port) is open
   - Check iptables/firewalld rules

---

## Provider-Specific Issues

### Docker

**Common Issues:**

1. **Socket permission denied**
   ```
   Error: permission denied while trying to connect to the Docker daemon socket
   ```

   **Solution:**
   ```yaml
   volumes:
     - /var/run/docker.sock:/var/run/docker.sock:ro
   user: root  # Or add user to docker group
   ```

2. **Container not found**
   - Verify container has `sablier.enable=true` label
   - Check container is not in another Docker network
   - Ensure container names match exactly

3. **Cannot scale containers**
   - Docker Classic doesn't support replicas
   - For scaling, use Docker Swarm or Kubernetes

**Debug Commands:**
```bash
# List containers with Sablier labels
docker ps -a --filter "label=sablier.enable=true"

# Check container labels
docker inspect <container> | jq '.[0].Config.Labels'

# Test socket access
docker ps
```

---

### Docker Swarm

**Common Issues:**

1. **Service not scaling**
   - Verify service mode is `replicated`, not `global`
   - Check service has `sablier.enable=true` label
   - Ensure sufficient node resources

2. **Wrong replica count**
   ```yaml
   services:
     myservice:
       deploy:
         replicas: 0  # Must be 0 for Sablier to manage
         labels:
           - sablier.enable=true
           - sablier.group=mygroup
   ```

3. **Manager node not accessible**
   - Sablier must connect to a Swarm manager node
   - Verify Docker socket points to manager

**Debug Commands:**
```bash
# List services
docker service ls

# Check service labels
docker service inspect <service> | jq '.[0].Spec.Labels'

# View service logs
docker service logs <service>
```

---

### Kubernetes

**Common Issues:**

1. **RBAC permissions**
   ```yaml
   apiVersion: rbac.authorization.k8s.io/v1
   kind: ClusterRole
   metadata:
     name: sablier
   rules:
   - apiGroups: ["apps"]
     resources: ["deployments", "statefulsets"]
     verbs: ["get", "list", "update", "patch"]
   - apiGroups: [""]
     resources: ["pods"]
     verbs: ["get", "list"]
   ```

2. **Namespace issues**
   - Sablier needs proper namespace configuration
   - Use delimiter to specify namespace in group name
   ```yaml
   provider:
     kubernetes:
       delimiter: "_"  # group_namespace
   ```

3. **Deployment not scaling**
   - Check deployment exists in specified namespace
   - Verify labels are correct
   - Ensure HPA is not conflicting

4. **kubeconfig issues**
   - In-cluster: Ensure ServiceAccount token is mounted
   - Out-of-cluster: Verify kubeconfig path

**Debug Commands:**
```bash
# Check deployments with Sablier labels
kubectl get deployments -l sablier.enable=true --all-namespaces

# View deployment labels
kubectl get deployment <name> -o yaml | grep -A5 labels

# Check Sablier pod logs
kubectl logs <sablier-pod> -n <namespace>

# Verify RBAC
kubectl auth can-i update deployments --as=system:serviceaccount:default:sablier
```

---

### Podman

**Common Issues:**

1. **Socket connection issues**
   ```yaml
   provider:
     podman:
       uri: unix:///run/podman/podman.sock
   ```

   **Verify socket exists:**
   ```bash
   ls -l /run/podman/podman.sock
   podman info
   ```

2. **Rootless vs root**
   - Socket location differs between rootless and root
   - Rootless: `unix://$XDG_RUNTIME_DIR/podman/podman.sock`
   - Root: `unix:///run/podman/podman.sock`

3. **Labels not working**
   - Ensure Podman version supports labels
   - Verify labels using `podman inspect`

**Debug Commands:**
```bash
# List containers with labels
podman ps -a --filter "label=sablier.enable=true"

# Check socket
podman --remote info

# Test connection
curl --unix-socket /run/podman/podman.sock http://localhost/v1.0.0/libpod/info
```

---

## Reverse Proxy Issues

### Traefik

**Common Issues:**

1. **Plugin not loaded**
   - Verify plugin is in Traefik static configuration
   - Check plugin version compatibility

2. **Middleware not applied**
   ```yaml
   http:
     middlewares:
       sablier:
         plugin:
           sablier:
             sablierUrl: http://sablier:10000
             group: mygroup
     routers:
       my-router:
         rule: "Host(`example.com`)"
         middlewares:
           - sablier  # Apply middleware
   ```

3. **Cannot reach Sablier API**
   - Verify Traefik and Sablier are on same Docker network
   - Check sablierUrl is correct

**For full Traefik troubleshooting, see:** [sablier-traefik-plugin](https://github.com/sablierapp/sablier-traefik-plugin)

---

### Caddy

**Common Issues:**

1. **Module not built**
   - Caddy requires rebuilding with Sablier module
   ```bash
   xcaddy build --with github.com/sablierapp/sablier-caddy-plugin
   ```

2. **Caddyfile syntax errors**
   ```caddyfile
   example.com {
       reverse_proxy @started whoami:80
       sablier {
           url http://sablier:10000
           group whoami
       }
   }
   ```

3. **Named matcher issues**
   - `@started` matcher must be defined by Sablier directive

**For full Caddy troubleshooting, see:** [sablier-caddy-plugin](https://github.com/sablierapp/sablier-caddy-plugin)

---

### Nginx

**Common Issues:**

1. **WASM module not loaded**
   - Nginx must be built with WASM support
   - Verify module is loaded in nginx.conf

2. **Subrequest buffer size**
   ```nginx
   subrequest_output_buffer_size 32k;  # Increase if needed
   ```

3. **Resolver configuration**
   ```nginx
   resolver 127.0.0.11 valid=10s ipv6=off;  # For Docker DNS
   ```

4. **Internal location not configured**
   ```nginx
   location /sablier/ {
       internal;
       proxy_method GET;
       proxy_pass http://sablier:10000/;
   }
   ```

**For full Nginx troubleshooting, see:** [sablier-proxywasm-plugin](https://github.com/sablierapp/sablier-proxywasm-plugin)

---

### Istio / Envoy

**Common Issues:**

1. **EnvoyFilter not applied**
   - Verify EnvoyFilter is in correct namespace
   - Check workloadSelector matches

2. **WASM plugin not loaded**
   - Ensure plugin image is accessible
   - Check Envoy logs for plugin errors

3. **Service mesh configuration**
   - Verify Sablier can reach workloads through mesh
   - Check mTLS settings

**For full Istio/Envoy troubleshooting, see:** [sablier-proxywasm-plugin](https://github.com/sablierapp/sablier-proxywasm-plugin)

---

### Apache APISIX

**Common Issues:**

1. **Plugin not enabled**
   - Verify plugin is enabled in APISIX configuration
   - Check plugin configuration in routes

2. **Cannot communicate with Sablier**
   - Verify APISIX can reach Sablier API
   - Check network configuration

**For full Apache APISIX troubleshooting, see:** [sablier-proxywasm-plugin](https://github.com/sablierapp/sablier-proxywasm-plugin)

---

## Configuration Issues

### Configuration Not Loaded

**Symptoms:**
- Sablier uses default values instead of your configuration
- Changes to config file don't take effect

**Solutions:**

1. **Verify config file location:**
   ```bash
   sablier start --configFile=/path/to/sablier.yaml
   ```

2. **Check file permissions:**
   ```bash
   ls -l /etc/sablier/sablier.yaml
   chmod 644 /etc/sablier/sablier.yaml
   ```

3. **Validate YAML syntax:**
   ```bash
   # Use a YAML validator
   yamllint sablier.yaml
   ```

4. **Check configuration precedence:**
   - Command-line args override everything
   - Environment variables override config file
   - Ensure you're not accidentally overriding via env vars

---

### Environment Variables Not Working

**Symptoms:**
- Environment variables don't set configuration values

**Solution:**

Use correct naming format:
```bash
# Incorrect
PROVIDER_NAME=docker

# Correct (all caps, underscore separated)
PROVIDER_NAME=docker
LOGGING_LEVEL=debug
SESSIONS_DEFAULT_DURATION=5m
STRATEGY_DYNAMIC_DEFAULT_THEME=matrix
```

**Nested configuration:**
```yaml
# Config file
strategy:
  dynamic:
    custom-themes-path: /themes
```

```bash
# Environment variable
STRATEGY_DYNAMIC_CUSTOM_THEMES_PATH=/themes
```

---

### Invalid Log Level

**Symptoms:**
- Warning: "invalid log level, defaulting to info"

**Solution:**

Use valid log levels (case-insensitive):
- `DEBUG`
- `INFO`
- `WARN`
- `ERROR`

**Example:**
```yaml
logging:
  level: debug  # Not "DEBUG" or "Debug"
```

---

## Performance Issues

### High CPU Usage

**Symptoms:**
- Sablier process using excessive CPU
- System becomes slow

**Possible Causes & Solutions:**

1. **Expiration interval too short**
   ```yaml
   sessions:
     expiration-interval: 20s  # Default
   ```

   **Solution:**
   - Increase interval if you use long session durations
   - Example: For 1h sessions, use 5m expiration interval
   ```yaml
   sessions:
     expiration-interval: 5m
   ```

2. **Too many active sessions**
   - Monitor number of active sessions
   - Consider session cleanup
   - Increase resources

3. **Logging level too verbose**
   - Debug logging impacts performance
   - Use `info` or `warn` in production

---

### Memory Leaks

**Symptoms:**
- Memory usage continuously grows
- OOM kills

**Debug Steps:**

1. **Enable state file to check sessions:**
   ```yaml
   storage:
     file: /var/lib/sablier/state.json
   ```

2. **Monitor sessions:**
   - Check if sessions are expiring correctly
   - Verify expiration interval is working

3. **Check for stuck goroutines:**
   - Enable pprof if available
   - Look for resource leaks

4. **Update to latest version:**
   - Memory leaks may be fixed in newer versions
   - Check release notes

---

## Getting More Help

### Before Asking for Help

1. **Enable debug logging**
2. **Collect relevant information:**
   - Sablier version: `sablier version`
   - Provider type and version
   - Reverse proxy type and version
   - Configuration files (remove sensitive data)
   - Error messages and logs
   - Steps to reproduce

### Support Channels

1. **Discord** (Real-time support)
   - Join: [Discord Server](https://discord.gg/WXYp59KeK9)
   - Best for: Quick questions, general discussion

2. **GitHub Issues** (Bug reports, feature requests)
   - Create issue: [GitHub Issues](https://github.com/sablierapp/sablier/issues/new)
   - Best for: Bugs, feature requests, detailed problems

3. **GitHub Discussions** (Q&A, ideas)
   - Start discussion: [GitHub Discussions](https://github.com/sablierapp/sablier/discussions)
   - Best for: How-to questions, ideas, feedback

4. **Documentation**
   - Read: [https://sablierapp.dev](https://sablierapp.dev)
   - Check guides and examples

### Plugin-Specific Issues

If your issue is specific to a reverse proxy plugin, please open an issue in the appropriate repository:

- **Traefik**: [sablier-traefik-plugin](https://github.com/sablierapp/sablier-traefik-plugin/issues)
- **Caddy**: [sablier-caddy-plugin](https://github.com/sablierapp/sablier-caddy-plugin/issues)
- **Proxy-WASM** (APISIX, Envoy, Istio, Nginx): [sablier-proxywasm-plugin](https://github.com/sablierapp/sablier-proxywasm-plugin/issues)

---

**Still having issues?** Don't hesitate to reach out on [Discord](https://discord.gg/WXYp59KeK9) or [open an issue](https://github.com/sablierapp/sablier/issues). We're here to help! 🚀
