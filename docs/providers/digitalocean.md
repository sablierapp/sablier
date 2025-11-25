# Digital Ocean

The Digital Ocean provider integrates with Digital Ocean's App Platform to scale apps on demand.

## Use the Digital Ocean provider

In order to use the Digital Ocean provider you can configure the [provider.name](../configuration) property along with your Digital Ocean API token.

<!-- tabs:start -->

#### **File (YAML)**

```yaml
provider:
  name: digitalocean
  digitalocean:
    token: your-digitalocean-api-token
    region: nyc1  # Optional, defaults to nyc1
```

#### **CLI**

```bash
sablier start --provider.name=digitalocean --provider.digitalocean.token=your-digitalocean-api-token
```

#### **Environment Variable**

```bash
PROVIDER_NAME=digitalocean
PROVIDER_DIGITALOCEAN_TOKEN=your-digitalocean-api-token
PROVIDER_DIGITALOCEAN_REGION=nyc1
```

<!-- tabs:end -->

!> **Keep your Digital Ocean API token secure! Never commit it to version control.**

## Register Apps

For Sablier to work, it needs to know which Digital Ocean apps to scale.

You register your apps by adding environment variables to your app specification.

```yaml
spec:
  name: my-app
  services:
    - name: web
      instance_count: 1
      envs:
        - key: SABLIER_ENABLE
          value: "true"
        - key: SABLIER_GROUP
          value: "mygroup"
```

## How does Sablier know when an app is ready?

Sablier monitors the deployment phase of your Digital Ocean app. An app is considered ready when:

- The active deployment is in the `ACTIVE` phase
- Instance count is greater than 0

Apps are considered not ready during:
- `PENDING_BUILD`
- `BUILDING`
- `PENDING_DEPLOY`
- `DEPLOYING`

Apps are in an unrecoverable state during:
- `ERROR`
- `CANCELED`

## Configuration Options

### Digital Ocean API Token

```yaml
provider:
  digitalocean:
    token: your-digitalocean-api-token
```

**Required.** Your Digital Ocean API token for authentication. You can generate a token in the Digital Ocean control panel under API > Tokens.

### Region

```yaml
provider:
  digitalocean:
    region: nyc1
```

**Optional.** The Digital Ocean region to use. Defaults to `nyc1`. While the API is global, this setting may be used for future region-specific features.

### Auto-stop on Startup

```yaml
provider:
  auto-stop-on-startup: true
```

When enabled, Sablier will scale down all apps with `SABLIER_ENABLE=true` environment variable that are running when Sablier starts.

## App Environment Variables

| Variable | Required | Description | Example |
|----------|----------|-------------|---------|
| `SABLIER_ENABLE` | Yes | Enable Sablier management for this app | `true` |
| `SABLIER_GROUP` | No | Logical group name for the app | `myapp` |

## How Scaling Works

### Starting an App

When Sablier receives a request to start an app:

1. It retrieves the current app specification
2. Updates the `instance_count` for all services and workers to 1 (or their previous value if it was already > 0)
3. Triggers a new deployment with the updated specification

### Stopping an App

When Sablier needs to stop an app:

1. It retrieves the current app specification
2. Sets the `instance_count` for all services and workers to 0
3. Triggers a new deployment with the updated specification

## Event Monitoring

Unlike Docker, Digital Ocean doesn't provide a real-time event stream. Sablier polls the App Platform API every 30 seconds to detect when apps are stopped (scaled to 0 instances).

## Limitations

- Polling-based event detection (30-second interval)
- Requires a valid Digital Ocean API token
- Only works with Digital Ocean App Platform (not Droplets, Kubernetes, or other services)
- Scaling operations trigger full deployments, which may take several minutes
- App identification uses App ID, not human-readable names

## Cost Considerations

⚠️ **Important:** Be aware of Digital Ocean App Platform pricing:

- Apps are billed per hour when running
- Scaling to 0 instances stops billing for compute resources
- Deployments may incur brief charges even when scaling down
- Database and storage resources may have separate billing

Sablier helps reduce costs by automatically scaling apps to 0 when not in use.

## Troubleshooting

### App not starting

1. Check Sablier logs for API errors
2. Verify your Digital Ocean API token is valid and has the correct permissions
3. Ensure the app exists and is accessible via the API
4. Check the app's deployment status in the Digital Ocean console

### Authentication errors

1. Verify your token in the Digital Ocean console
2. Ensure the token has read/write permissions for App Platform
3. Check that the token hasn't expired

### Slow scaling

Digital Ocean App Platform deployments can take several minutes:
- Building the app (if code changes were made)
- Deploying new instances
- Health checks

This is expected behavior. Consider adjusting your Sablier timeout settings accordingly.

## Security Best Practices

1. **Store tokens securely**: Use environment variables or secrets management
2. **Use scoped tokens**: Create tokens with minimal required permissions
3. **Rotate tokens regularly**: Update API tokens periodically
4. **Monitor API usage**: Check Digital Ocean console for unexpected API calls

## Full Example

A complete example would include:

1. A Digital Ocean app with `SABLIER_ENABLE=true` environment variable
2. Sablier running with Digital Ocean provider configuration
3. A reverse proxy (Traefik, Nginx, etc.) configured to use Sablier's API

See the Digital Ocean provider example (if available) for a complete setup.
