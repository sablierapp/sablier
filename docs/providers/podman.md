# Podman

The Podman provider communicates with the `podman.sock` socket to start and stop containers on demand.

## Use the Podman provider

In order to use the docker provider you can configure the [provider.name](../configuration) property.

<!-- tabs:start -->

#### **File (YAML)**

```yaml
provider:
  name: podman
```

#### **CLI**

```bash
sablier start --provider.name=podman
```

#### **Environment Variable**

```bash
PROVIDER_NAME=podman
```

<!-- tabs:end -->

!> **Ensure that Sablier has access to the podman socket!**

<!-- x-release-please-start-version -->
```yaml
services:
  sablier:
    image: sablierapp/sablier:1.10.1
    command:
      - start
      - --provider.name=podman
    volumes:
      - '/run/podman/podman.sock:/run/podman/podman.sock'
```
<!-- x-release-please-end -->

## Register containers

For Sablier to work, it needs to know which podman container to start and stop.

You have to register your containers by opting-in with labels.

```yaml
services:
  whoami:
    image: acouvreur/whoami:v1.10.2
    labels:
      - sablier.enable=true
      - sablier.group=mygroup
```

## How does Sablier know when a container is ready?

If the container defines a Healthcheck, then Sablier will check for healthiness before marking the container as `ready`.

If the container does not define a Healthcheck, then as soon as the container has the status `started`, it is considered ready.

## Configuration Options

### Podman Socket URI

```yaml
provider:
  name: podman
  podman:
    uri: unix:///run/podman/podman.sock
```

The socket URI depends on your Podman setup:

| Mode | Default Socket Path | URI |
|------|---------------------|-----|
| Rootful | `/run/podman/podman.sock` | `unix:///run/podman/podman.sock` |
| Rootless | `$XDG_RUNTIME_DIR/podman/podman.sock` | `unix:///run/user/1000/podman/podman.sock` |

### Enabling the Podman Socket

#### Rootless Mode (Recommended)

```bash
# Enable the socket for the current user
systemctl --user enable podman.socket
systemctl --user start podman.socket

# Verify it's running
systemctl --user status podman.socket

# Check socket location
echo $XDG_RUNTIME_DIR
ls -l $XDG_RUNTIME_DIR/podman/podman.sock
```

Configuration for rootless:
```yaml
provider:
  podman:
    uri: unix:///run/user/1000/podman/podman.sock
```

#### Rootful Mode

```bash
# Enable the socket as root
sudo systemctl enable podman.socket
sudo systemctl start podman.socket

# Verify it's running
sudo systemctl status podman.socket

# Check socket location
sudo ls -l /run/podman/podman.sock
```

Configuration for rootful:
```yaml
provider:
  podman:
    uri: unix:///run/podman/podman.sock
```

### Auto-stop on Startup

```yaml
provider:
  auto-stop-on-startup: true
```

When enabled, Sablier will stop all containers with `sablier.enable=true` label that are running but not registered in an active session when Sablier starts.

## Container Labels

| Label | Required | Description | Example |
|-------|----------|-------------|---------|
| `sablier.enable` | Yes | Enable Sablier management for this container | `true` |
| `sablier.group` | Yes | Logical group name for the container | `myapp` |

## Using with Podman Compose

Podman supports Docker Compose files via `podman-compose`:

```yaml
version: '3.8'

services:
  sablier:
    image: docker.io/sablierapp/sablier:1.10.1
    command:
      - start
      - --provider.name=podman
      - --provider.podman.uri=unix:///run/podman/podman.sock
    volumes:
      - /run/podman/podman.sock:/run/podman/podman.sock
    networks:
      - sablier-network
    ports:
      - "10000:10000"

  myapp:
    image: docker.io/myapp:latest
    labels:
      - sablier.enable=true
      - sablier.group=myapp
    networks:
      - sablier-network
```

### Installing podman-compose

```bash
# Using pip
pip install podman-compose

# Or using pipx
pipx install podman-compose

# Verify installation
podman-compose --version
```

## Full Example

See the [Podman provider example](../../examples/podman/) for a complete, working setup.

## Rootless vs Rootful

### Rootless Mode (Recommended)

**Advantages:**
- Better security (runs as regular user)
- No root privileges required
- User-specific containers

**Considerations:**
- Socket path is user-specific
- Must configure `$XDG_RUNTIME_DIR` correctly
- May need to handle user namespaces

### Rootful Mode

**Advantages:**
- Traditional Docker-like behavior
- System-wide containers
- Simpler socket path

**Considerations:**
- Requires root privileges
- Less isolated
- System-wide impact

## Limitations

- Requires Podman socket to be enabled and accessible
- Cannot manage containers on remote Podman hosts
- Healthchecks must be defined in the container image or compose file
- podman-compose has some limitations compared to docker-compose

## SELinux Considerations

On systems with SELinux enabled (like Red Hat, Fedora, CentOS):

### For Rootful Mode

Set the correct SELinux context:
```bash
sudo chcon -t container_runtime_exec_t /run/podman/podman.sock
```

### For Rootless Mode

Usually works without additional configuration, but if you encounter issues:
```bash
semanage fcontext -a -t container_runtime_exec_t "$XDG_RUNTIME_DIR/podman/podman.sock"
restorecon -v "$XDG_RUNTIME_DIR/podman/podman.sock"
```

## Troubleshooting

### Socket Not Found

```bash
# Check if socket service is running
systemctl --user status podman.socket   # For rootless
sudo systemctl status podman.socket      # For rootful

# Enable and start if not running
systemctl --user enable --now podman.socket   # For rootless
sudo systemctl enable --now podman.socket      # For rootful
```

### Permission Denied

For rootless mode, ensure `$XDG_RUNTIME_DIR` is set:
```bash
echo $XDG_RUNTIME_DIR
# Should output something like: /run/user/1000
```

For rootful mode, ensure Sablier has access to the socket:
```bash
sudo ls -l /run/podman/podman.sock
```

### Container Not Starting

1. Check Sablier logs
2. Verify container labels
3. Test socket connectivity:
   ```bash
   curl --unix-socket /run/podman/podman.sock http://localhost/_ping
   ```

### podman-compose Not Working

Ensure you're using compatible compose file syntax:
```yaml
version: '3.8'  # Supported version

# Use docker.io/ prefix for images
image: docker.io/sablierapp/sablier:1.10.1
```

### Rootless Networking Issues

Podman rootless uses `slirp4netns` by default. For better performance, consider using `pasta`:
```bash
# Configure pasta for rootless networking
podman system connection add pasta --default
```