# Sablier Provider Examples

This directory contains complete, runnable examples for each Sablier provider. Each example is fully documented and includes automated tests.

## Available Examples

### [Docker](./docker/)
Demonstrates Sablier with the Docker provider, managing individual containers on demand.

**Features:**
- Container start/stop on demand
- Dynamic and blocking strategies
- Docker socket integration
- Automated testing

**Quick Start:**
```bash
cd docker
make test
```

### [Docker Swarm](./docker-swarm/)
Shows Sablier managing Docker Swarm services with replica scaling.

**Features:**
- Service replica scaling (0 to N)
- Swarm mode configuration
- Native healthcheck support
- Stack deployment

**Quick Start:**
```bash
cd docker-swarm
make test
```

### [Kubernetes](./kubernetes/)
Complete Kubernetes deployment with RBAC, ConfigMaps, and Ingress.

**Features:**
- Deployment scaling
- RBAC configuration
- ConfigMap-based configuration
- Ingress routing
- StatefulSet support

**Quick Start:**
```bash
cd kubernetes
make test
```

### [Nomad](./nomad/)
HashiCorp Nomad integration with task group scaling and service discovery.

**Features:**
- Task group scaling (0 to N)
- Nomad service discovery
- ACL token support
- Multi-region support
- Event stream monitoring

**Quick Start:**
```bash
cd nomad
make test
```

### [Podman](./podman/)
Podman provider example with both rootless and rootful modes.

**Features:**
- Rootless container management
- Podman socket integration
- podman-compose support
- SELinux compatibility

**Quick Start:**
```bash
cd podman
make socket-enable  # Required first time
make test
```

## Common Structure

Each example follows a consistent structure:

```
provider/
├── README.md           # Complete documentation
├── Makefile           # Build and test automation
├── sablier.yaml       # Sablier configuration
├── nginx.conf         # Reverse proxy config
└── ...                # Provider-specific files
```

## Testing

Each example includes a Makefile with automated tests:
1. Sets up the environment
2. Deploys Sablier and demo applications
3. Verifies container/service scaling
4. Tests both dynamic and blocking strategies
5. Cleans up resources

### Running Tests Locally

```bash
# Test a specific provider
cd docker
make test

# Or manually control
make up      # Start services
make status  # Check status
make down    # Stop services
```

### CI/CD

All examples are automatically tested in GitHub Actions on:
- Pull requests
- Pushes to main branch
- Weekly schedule (to catch regressions)

See [.github/workflows/examples.yml](../.github/workflows/examples.yml)

## Configuration

All examples use similar Sablier configurations with provider-specific adjustments:

```yaml
provider:
  name: <docker|docker_swarm|kubernetes|nomad|podman>
  # Provider-specific settings

server:
  port: 10000
  base-path: /

sessions:
  default-duration: 1m       # Short for testing
  expiration-interval: 10s

logging:
  level: debug              # Verbose for examples

strategy:
  dynamic:
    display-name: "Example"
    show-details-by-default: true
    default-theme: hacker-terminal
    default-refresh-frequency: 5s
  blocking:
    default-timeout: 1m
    default-refresh-frequency: 1s
```

## Strategies Demonstrated

### Dynamic Strategy
Shows a loading page while containers/services start:
```
http://localhost:8080/whoami
→ Returns loading page immediately
→ Starts container in background
→ Auto-refreshes until ready
```

### Blocking Strategy
Waits for containers/services to be ready before responding:
```
http://localhost:8080/blocking
→ Waits for container to start
→ Returns actual service response
→ May timeout if startup takes too long
```

## Resource Labels

All examples use consistent labeling:

**Docker/Podman:**
```yaml
labels:
  - sablier.enable=true
  - sablier.group=whoami
```

**Docker Swarm:**
```yaml
deploy:
  labels:
    - sablier.enable=true
    - sablier.group=whoami
```

**Kubernetes:**
```yaml
metadata:
  labels:
    sablier.enable: "true"
    sablier.group: whoami
```

**Nomad:**
```hcl
group "whoami" {
  meta {
    "sablier.enable" = "true"
    "sablier.group"  = "whoami"
  }
}
```

## Troubleshooting

### General Issues

1. **Check Sablier health:**
   ```bash
   curl http://localhost:10000/health
   ```

2. **View logs:**
   ```bash
   # Docker/Podman
   cd docker  # or cd podman
   make logs-sablier
   
   # Swarm
   cd docker-swarm
   make logs
   
   # Kubernetes
   cd kubernetes
   make logs
   ```

3. **Verify labels:**
   ```bash
   # Docker
   docker inspect whoami | grep -A 5 Labels
   
   # Kubernetes
   kubectl get deployment whoami -o yaml | grep -A 5 labels
   ```

### Provider-Specific Issues

See the README.md in each provider directory for detailed troubleshooting.

## Learn More

- [Sablier Documentation](../docs/)
- [Provider Documentation](../docs/providers/)
- [Configuration Reference](../docs/configuration.md)
- [Strategies](../docs/strategies.md)

## Contributing

To add a new example:

1. Create a new directory: `examples/new-provider/`
2. Include all necessary files (README.md, config, test.sh)
3. Add to CI workflow in `.github/workflows/examples.yml`
4. Update this README with the new example
5. Test locally before submitting PR

## License

All examples are provided under the same license as the Sablier project. See [LICENSE](../LICENSE).
