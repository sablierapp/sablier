# Nomad

The Nomad provider allows Sablier to manage [HashiCorp Nomad](https://www.nomadproject.io/) job task groups, scaling them from 0 to N allocations on demand.

## Overview

Sablier integrates with Nomad to:
- Scale task groups to zero when idle
- Scale task groups up on first request
- Monitor allocation health and readiness

## Use the Nomad Provider

Configure the provider name as `nomad`:

<!-- tabs:start -->

#### **File (YAML)**

```yaml
provider:
  name: nomad
  nomad:
    address: "http://127.0.0.1:4646"
    namespace: "default"
    token: ""  # Optional ACL token
    region: "" # Optional region
```

#### **CLI**

```bash
sablier start --provider.name=nomad \
  --provider.nomad.address=http://127.0.0.1:4646 \
  --provider.nomad.namespace=default
```

#### **Environment Variables**

```bash
PROVIDER_NAME=nomad
PROVIDER_NOMAD_ADDRESS=http://127.0.0.1:4646
PROVIDER_NOMAD_NAMESPACE=default
PROVIDER_NOMAD_TOKEN=your-acl-token
PROVIDER_NOMAD_REGION=us-east-1
```

<!-- tabs:end -->

## Configuration

### Connection Settings

| Setting    | Description                                             | Default                  | Environment Variable    |
|------------|---------------------------------------------------------|--------------------------|-------------------------|
| `address`  | HTTP address of the Nomad server                        | `http://127.0.0.1:4646` | `NOMAD_ADDR`            |
| `token`    | Secret ID of an ACL token (if ACLs are enabled)         | `""`                     | `NOMAD_TOKEN`           |
| `namespace`| Target namespace for operations                         | `default`                | `NOMAD_NAMESPACE`       |
| `region`   | Target region for operations                            | `""`                     | `NOMAD_REGION`          |

### Example Configuration

```yaml
provider:
  name: nomad
  nomad:
    address: "https://nomad.example.com:4646"
    namespace: "production"
    token: "your-secret-acl-token"
    region: "us-west-2"

server:
  port: 10000

sessions:
  default-duration: 5m

strategy:
  dynamic:
    default-theme: "hacker-terminal"
```

## Labeling Jobs

Mark task groups for Sablier management using metadata:

```hcl
job "whoami" {
  datacenters = ["dc1"]
  
  group "web" {
    count = 0  # Start at 0
    
    meta {
      sablier.enable = "true"
      sablier.group  = "whoami"  # Optional group name
    }
    
    task "server" {
      driver = "docker"
      
      config {
        image = "containous/whoami"
        ports = ["http"]
      }
      
      resources {
        cpu    = 100
        memory = 128
      }
    }
    
    network {
      port "http" {
        to = 80
      }
    }
  }
}
```

### Required Labels

| Label | Value | Description |
|-------|-------|-------------|
| `sablier.enable` | `"true"` | Enables Sablier management for this task group |

### Optional Labels

| Label | Value | Description |
|-------|-------|-------------|
| `sablier.group` | `string` | Group name for managing multiple task groups together (default: `"default"`) |

## Instance Naming

Nomad instances are identified using the format: `jobID/taskGroupName`

Examples:
- `whoami/web` - Task group "web" in job "whoami"
- `api/backend` - Task group "backend" in job "api"

If you only provide the job ID (e.g., `whoami`), Sablier will assume the task group has the same name as the job.

## Reverse Proxy Integration

### Traefik

```yaml
http:
  middlewares:
    sablier-whoami:
      plugin:
        sablier:
          sablierUrl: http://sablier:10000
          names: "whoami/web"
          sessionDuration: 1m
          
  routers:
    whoami:
      rule: "Host(`whoami.localhost`)"
      middlewares:
        - sablier-whoami
      service: whoami
      
  services:
    whoami:
      loadBalancer:
        servers:
          - url: "http://whoami.service.consul:80"
```

### Nginx

```nginx
location / {
    set $sablierUrl 'http://sablier:10000';
    set $sablierNames 'whoami/web';
    set $sablierSessionDuration '1m';
    set $sablierNginxInternalRedirect '@whoami';
    
    js_content sablier.call;
}

location @whoami {
    proxy_pass http://whoami.service.consul;
}
```

## Scaling Behavior

### Scale Up (0 → N)

When a request arrives for a scaled-down task group:

1. Sablier updates the job's task group `count` to the desired value (default: 1)
2. Nomad scheduler places allocations on available nodes
3. Allocations transition through: `pending` → `running`
4. If health checks are configured, Sablier waits for them to pass
5. Once all allocations are healthy, the instance is marked as `ready`

### Scale Down (N → 0)

When the session expires:

1. Sablier updates the job's task group `count` to 0
2. Nomad gracefully stops all allocations
3. The instance is marked as `not-ready`

## Health Checks

Sablier respects Nomad's deployment health checks. If your task group has health checks configured, Sablier will wait for allocations to be marked as healthy before considering the instance ready.

Example with Consul health checks:

```hcl
group "web" {
  count = 0
  
  meta {
    sablier.enable = "true"
  }
  
  task "server" {
    driver = "docker"
    
    service {
      name = "whoami"
      port = "http"
      
      check {
        type     = "http"
        path     = "/"
        interval = "10s"
        timeout  = "2s"
      }
    }
  }
}
```

## Permissions (ACL)

If Nomad ACLs are enabled, the token must have the following permissions:

```hcl
namespace "default" {
  policy = "write"
  
  capabilities = [
    "read-job",
    "submit-job",
    "dispatch-job",
    "read-logs",
    "read-fs",
    "alloc-node-exec",
    "list-jobs",
    "parse-job",
  ]
}

node {
  policy = "read"
}
```

Create a policy file `sablier-policy.hcl`:

```hcl
namespace "default" {
  policy = "write"
}
```

Apply it:

```bash
nomad acl policy apply sablier ./sablier-policy.hcl
nomad acl token create -name="sablier" -policy=sablier
```

Use the generated token's Secret ID in your Sablier configuration.

## Example Deployment

### Nomad Job for Sablier

```hcl
job "sablier" {
  datacenters = ["dc1"]
  type = "service"
  
  group "sablier" {
    count = 1
    
    network {
      port "http" {
        static = 10000
        to     = 10000
      }
    }
    
    task "sablier" {
      driver = "docker"
      
      config {
        image = "sablierapp/sablier:1.10.1"
        ports = ["http"]
        
        args = [
          "start",
          "--provider.name=nomad",
        ]
      }
      
      env {
        NOMAD_ADDR      = "http://nomad.service.consul:4646"
        NOMAD_NAMESPACE = "default"
        NOMAD_TOKEN     = "${NOMAD_TOKEN}"  # Pass via template
      }
      
      template {
        data = <<EOH
NOMAD_TOKEN="{{ with secret "secret/nomad/sablier" }}{{ .Data.data.token }}{{ end }}"
EOH
        destination = "secrets/env"
        env         = true
      }
      
      resources {
        cpu    = 200
        memory = 256
      }
      
      service {
        name = "sablier"
        port = "http"
        
        check {
          type     = "http"
          path     = "/health"
          interval = "10s"
          timeout  = "2s"
        }
      }
    }
  }
}
```

## Strategies

### Dynamic Strategy

Shows a loading page while task groups start:

```
http://sablier:10000/api/strategies/dynamic?names=whoami/web&session_duration=1m
```

The page auto-refreshes until allocations are running and healthy.

### Blocking Strategy

Waits for task groups to be ready before responding:

```
http://sablier:10000/api/strategies/blocking?names=whoami/web&session_duration=1m&timeout=60s
```

Returns the proxied response once allocations are healthy or times out.

## Limitations

- Only supports jobs with `count`-based scaling (not `percentage` based)
- Does not support `system` or `batch` job types (only `service` jobs)
- Task group must use the `count` field (not dynamic application sizing)
- Event stream requires Nomad 1.0+ for real-time notifications

## Troubleshooting

### "Cannot connect to nomad"

Check that the Nomad address is correct and accessible:

```bash
curl http://127.0.0.1:4646/v1/status/leader
```

### "Job not found"

Ensure the job exists in the specified namespace:

```bash
nomad job status -namespace=default whoami
```

### "Task group not found"

Verify the task group name matches:

```bash
nomad job inspect -namespace=default whoami | jq '.Job.TaskGroups[].Name'
```

### "Forbidden" errors

Check ACL token permissions:

```bash
nomad acl token self
```

### Allocations not starting

Check Nomad scheduler:

```bash
nomad job status whoami
nomad alloc status <alloc-id>
```

## Further Reading

- [Nomad Documentation](https://www.nomadproject.io/docs)
- [Nomad API Reference](https://www.nomadproject.io/api-docs)
- [Nomad ACL System](https://www.nomadproject.io/docs/operations/acl)
- [Nomad Job Specification](https://www.nomadproject.io/docs/job-specification)
