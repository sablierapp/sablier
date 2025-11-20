# Digital Ocean Provider Integration Tests

This directory contains integration tests for the Digital Ocean provider. These tests interact with the real Digital Ocean API and require valid credentials.

## Prerequisites

1. **Digital Ocean Account**: You need an active Digital Ocean account
2. **API Token**: Generate a personal access token with read/write permissions for App Platform
3. **Test App**: Create a minimal Digital Ocean app with the following environment variables:
   - `SABLIER_ENABLE=true`
   - `SABLIER_GROUP=test` (optional)

## Running the Tests

### Set Environment Variables

**PowerShell:**
```powershell
$env:DIGITALOCEAN_TOKEN="your-digitalocean-api-token"
$env:DIGITALOCEAN_TEST_APP_ID="your-app-id"
```

**Bash/Linux:**
```bash
export DIGITALOCEAN_TOKEN="your-digitalocean-api-token"
export DIGITALOCEAN_TEST_APP_ID="your-app-id"
```

### Run Tests

```bash
# Run all Digital Ocean provider tests
go test ./pkg/provider/digitalocean -v

# Run specific test
go test ./pkg/provider/digitalocean -v -run TestDigitalOceanProvider_InstanceStart

# Skip integration tests (runs only unit tests if any)
go test ./pkg/provider/digitalocean -v -short
```

## Test Behavior

- **Skipped if credentials not provided**: Tests will be skipped if `DIGITALOCEAN_TOKEN` or `DIGITALOCEAN_TEST_APP_ID` are not set
- **Cleanup**: Tests automatically clean up by scaling the app to 0 instances after completion
- **Deployment wait times**: Tests may take several minutes due to Digital Ocean deployment times
- **Polling-based events**: The `NotifyInstanceStopped` test may take up to 90 seconds due to 30-second polling interval

## Creating a Test App

The simplest way to create a test app:

1. Go to Digital Ocean Console → Apps
2. Create a new app (use a static site or simple container)
3. Add environment variables:
   - `SABLIER_ENABLE=true`
   - `SABLIER_GROUP=test`
4. Deploy the app
5. Copy the App ID from the URL or app details

**Note**: The test app should be minimal to reduce costs and deployment times.

## Cost Considerations

⚠️ **Warning**: Running these tests will:
- Trigger deployments on your Digital Ocean account
- May incur charges based on Digital Ocean App Platform pricing
- Scale apps up and down, which may result in brief compute charges

It's recommended to:
- Use the smallest instance size for your test app
- Run tests sparingly
- Use a test/development Digital Ocean account if possible

## Troubleshooting

### Tests are skipped
- Verify environment variables are set correctly
- Check that the API token is valid

### Tests timeout
- Digital Ocean deployments can take 3-5 minutes
- Increase timeout if needed for slower apps

### Authentication errors
- Verify your token has the correct permissions
- Check that the token hasn't expired

### App not found
- Verify the App ID is correct
- Ensure the app exists in your Digital Ocean account
