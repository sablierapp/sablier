package digitalocean_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/digitalocean/godo"
	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider/digitalocean"
)

// setupDigitalOcean creates a Digital Ocean client for testing
// Skips the test if DIGITALOCEAN_TOKEN is not set
func setupDigitalOcean(t *testing.T) (*digitalocean.Provider, *godo.Client) {
	t.Helper()

	token := os.Getenv("DIGITALOCEAN_TOKEN")
	if token == "" {
		t.Skip("DIGITALOCEAN_TOKEN environment variable not set, skipping Digital Ocean integration test")
	}

	ctx := context.Background()
	client := godo.NewFromToken(token)

	provider, err := digitalocean.New(ctx, client, slogt.New(t))
	if err != nil {
		t.Fatalf("failed to create Digital Ocean provider: %s", err)
	}

	return provider, client
}

// getTestAppID returns the app ID to use for testing
// Either from DIGITALOCEAN_TEST_APP_ID env var or creates a test app
func getTestAppID(t *testing.T, client *godo.Client) string {
	t.Helper()

	// Check if test app ID is provided
	appID := os.Getenv("DIGITALOCEAN_TEST_APP_ID")
	if appID != "" {
		return appID
	}

	t.Skip("DIGITALOCEAN_TEST_APP_ID environment variable not set. Please provide an existing app ID for testing, or create a minimal app with SABLIER_ENABLE=true environment variable.")
	return ""
}

// cleanupApp ensures the app is in a known state after testing
func cleanupApp(t *testing.T, client *godo.Client, appID string) {
	t.Helper()
	ctx := context.Background()

	// Get the app
	app, _, err := client.Apps.Get(ctx, appID)
	if err != nil {
		t.Logf("failed to get app for cleanup: %s", err)
		return
	}

	// Scale down to 0 if needed
	needsUpdate := false
	updateRequest := &godo.AppUpdateRequest{
		Spec: app.Spec,
	}

	for i := range updateRequest.Spec.Services {
		if updateRequest.Spec.Services[i].InstanceCount > 0 {
			updateRequest.Spec.Services[i].InstanceCount = 0
			needsUpdate = true
		}
	}

	for i := range updateRequest.Spec.Workers {
		if updateRequest.Spec.Workers[i].InstanceCount > 0 {
			updateRequest.Spec.Workers[i].InstanceCount = 0
			needsUpdate = true
		}
	}

	if needsUpdate {
		_, _, err = client.Apps.Update(ctx, appID, updateRequest)
		if err != nil {
			t.Logf("failed to cleanup app: %s", err)
		}
	}
}

// waitForDeployment waits for the app deployment to reach a specific phase
func waitForDeployment(ctx context.Context, t *testing.T, client *godo.Client, appID string, timeout time.Duration) error {
	t.Helper()

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			app, _, err := client.Apps.Get(ctx, appID)
			if err != nil {
				return err
			}

			if app.ActiveDeployment != nil {
				phase := app.ActiveDeployment.Phase
				t.Logf("Current deployment phase: %s", phase)

				switch phase {
				case "ACTIVE", "ERROR", "CANCELED":
					return nil
				}
			}

			if time.Now().After(deadline) {
				t.Log("Timeout waiting for deployment, continuing anyway...")
				return nil
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
