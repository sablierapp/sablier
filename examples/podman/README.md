# Podman Provider Example

This example demonstrates Sablier with the **Podman provider**, showcasing automatic scaling of rootless containers.

## Quick Start

```bash
# Enable Podman socket (required)
make socket-enable

# Start and test
make test

# Or start manually
make up

# Check status
make status

# View logs
make logs
```

Access the example at http://localhost:8080/whoami

## What's Inside

- **Sablier**: Scale-to-zero controller
- **whoami**: Demo service (scales from 0→1→0)
- **nginx**: Reverse proxy with Sablier integration

## How It Works

1. Request arrives at nginx (`:8080`)
2. Nginx calls Sablier API with session name
3. Sablier checks if `whoami` container is running via Podman socket
4. If stopped: Sablier starts container and waits
5. Nginx proxies request to `whoami`
6. After 1 minute idle: Sablier stops container

## Configuration

### Podman Socket

Sablier connects to Podman via Unix socket:
- **Rootless**: `$XDG_RUNTIME_DIR/podman/podman.sock` (default)
- **Rootful**: `/run/podman/podman.sock`

Enable with: `make socket-enable`

### Files

| File | Purpose |
|------|---------|
| `docker-compose.yml` | Service definitions |
| `sablier.yaml` | Sablier configuration |
| `nginx.conf` | Reverse proxy with Sablier headers |
| `Makefile` | Automation commands |

## Available Commands

| Command | Description |
|---------|-------------|
| `make help` | Show all commands |
| `make socket-enable` | Enable Podman socket |
| `make up` | Start services |
| `make down` | Stop and remove services |
| `make test` | Run automated tests |
| `make logs` | Show all logs |
| `make logs-sablier` | Show Sablier logs |
| `make logs-whoami` | Show whoami logs |
| `make health` | Check Sablier health |
| `make status` | Show container status |
| `make try` | Quick manual test |
| `make clean` | Clean up everything |

## Endpoints

| URL | Description |
|-----|-------------|
| `http://localhost:8080/whoami` | Dynamic strategy (returns immediately) |
| `http://localhost:8080/blocking` | Blocking strategy (waits for container) |
| `http://localhost:10000/health` | Sablier health check |

## Rootless vs Rootful

**Rootless (Recommended)**:
```bash
# Enable socket
systemctl --user enable --now podman.socket

# Socket location
echo $XDG_RUNTIME_DIR/podman/podman.sock
```

**Rootful (requires sudo)**:
```bash
# Enable socket
sudo systemctl enable --now podman.socket

# Socket location
/run/podman/podman.sock
```

The Makefile automatically detects your mode.

## Troubleshooting

**"Podman socket not found"**
```bash
make socket-enable
# Or manually:
systemctl --user enable --now podman.socket
```

**"Connection refused"**
```bash
# Check socket
ls -l $XDG_RUNTIME_DIR/podman/podman.sock

# Verify permissions
podman info | grep -A 5 "remoteSocket"
```

**Container won't start**
```bash
# Check Sablier logs
make logs-sablier

# Verify Podman connection
podman --remote info
```

**podman-compose not found**
```bash
# Install podman-compose
pip3 install podman-compose
# Or: dnf install podman-compose
```
