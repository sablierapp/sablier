# Nomad Provider Example

Automatically start and stop Nomad task groups on demand using Sablier.

## Quick Start

```bash
make up      # Start Nomad dev cluster and deploy jobs
make try     # Test it (accesses whoami via nginx)
make test    # Run automated tests
make down    # Clean up
```

## What's Inside

- **Nomad** - Job orchestrator (dev mode)
- **Sablier** - Task group lifecycle manager
- **Nginx** - Reverse proxy
- **Whoami** - Demo app (starts on first request, stops after 1min idle)

## How It Works

```
Request → Nginx → Sablier → Scales Task Group → Proxies Request
                            ↓ (1min idle)
                         Stops Task Group
```

## Configuration

**sablier.yaml**:
```yaml
provider:
  name: nomad
  nomad:
    address: http://nomad:4646
    namespace: default

sessions:
  default-duration: 1m        # Auto-stop after 1min idle
```

**Task Group Meta** (whoami.nomad.hcl):
```hcl
meta {
  "sablier.enable" = "true"
  "sablier.group"  = "whoami"
}
```

## Available Commands

| Command | Description |
|---------|-------------|
| `make up` | Start Nomad cluster and deploy jobs |
| `make down` | Stop and clean up everything |
| `make test` | Run automated tests |
| `make status` | Show job status |
| `make logs` | View all logs |
| `make logs-sablier` | View Sablier logs only |
| `make logs-nomad` | View Nomad logs only |
| `make health` | Check Sablier health |
| `make try` | Quick manual test |
| `make nomad-ui` | Open Nomad UI in browser |

## Endpoints

- **Dynamic Strategy**: http://localhost:8080/whoami  
  Shows loading page, auto-refreshes when ready
  
- **Blocking Strategy**: http://localhost:8080/blocking  
  Waits for task group, then shows response
  
- **Health Check**: http://localhost:10000/health
- **Nomad UI**: http://localhost:4646

## Troubleshooting

**Task group won't start?**
```bash
make logs-sablier                    # Check Sablier logs
make logs-nomad                      # Check Nomad logs
docker exec nomad nomad job status whoami  # Check job status
```

**Connection errors?**
```bash
docker exec nomad nomad status       # Verify Nomad is running
curl http://localhost:4646/v1/status/leader  # Check API
```

**Task group still running?**
- Session expires after 1 minute of inactivity
- Make a request to refresh the session

## Learn More

- [Nomad Provider Documentation](../../docs/providers/nomad.md)
- [Configuration Reference](../../docs/configuration.md)
- [All Examples](../)
