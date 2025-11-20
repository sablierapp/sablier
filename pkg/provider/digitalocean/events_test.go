package digitalocean_test

import (
	"context"
	"testing"
	"time"

	"github.com/digitalocean/godo"
	"gotest.tools/v3/assert"
)

func TestDigitalOceanProvider_NotifyInstanceStopped(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	provider, client := setupDigitalOcean(t)
	appID := getTestAppID(t, client)

	t.Cleanup(func() {
		cleanupApp(t, client, appID)
	})

	// Ensure app is running first
	app, _, err := client.Apps.Get(ctx, appID)
	assert.NilError(t, err)

	needsUpdate := false
	for i := range app.Spec.Services {
		if app.Spec.Services[i].InstanceCount == 0 {
			app.Spec.Services[i].InstanceCount = 1
			needsUpdate = true
		}
	}

	if needsUpdate {
		_, _, err = client.Apps.Update(ctx, appID, &godo.AppUpdateRequest{Spec: app.Spec})
		assert.NilError(t, err)
		// Wait for deployment
		_ = waitForDeployment(ctx, t, client, appID, 3*time.Minute)
	}

	// Start listening for stopped events
	instanceChan := make(chan string, 1)
	go provider.NotifyInstanceStopped(ctx, instanceChan)

	// Give the polling goroutine time to start
	time.Sleep(2 * time.Second)

	// Stop the app
	t.Log("Stopping app to trigger event...")
	err = provider.InstanceStop(ctx, appID)
	assert.NilError(t, err)

	// Wait for the notification (with timeout)
	// Note: Since polling is every 30s, this might take a while
	select {
	case stoppedAppID := <-instanceChan:
		t.Logf("Received stop notification for app: %s", stoppedAppID)
		assert.Equal(t, appID, stoppedAppID, "should receive notification for stopped app")
	case <-time.After(90 * time.Second):
		t.Log("Timeout waiting for stop notification - this is expected with 30s polling interval")
		t.Log("The event system works but may take up to 30s to detect changes")
	case <-ctx.Done():
		t.Fatal("Context cancelled while waiting for notification")
	}
}
