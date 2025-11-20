package digitalocean_test

import (
	"context"
	"testing"
	"time"

	"github.com/digitalocean/godo"
	"gotest.tools/v3/assert"
)

func TestDigitalOceanProvider_InstanceStop(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	provider, client := setupDigitalOcean(t)
	appID := getTestAppID(t, client)

	t.Cleanup(func() {
		cleanupApp(t, client, appID)
	})

	tests := []struct {
		name    string
		setup   func(t *testing.T)
		wantErr bool
	}{
		{
			name: "stop running app",
			setup: func(t *testing.T) {
				// Ensure app has at least 1 instance
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
			},
			wantErr: false,
		},
		{
			name: "stop already stopped app",
			setup: func(t *testing.T) {
				// Ensure app is at 0
				app, _, err := client.Apps.Get(ctx, appID)
				assert.NilError(t, err)

				needsUpdate := false
				for i := range app.Spec.Services {
					if app.Spec.Services[i].InstanceCount > 0 {
						app.Spec.Services[i].InstanceCount = 0
						needsUpdate = true
					}
				}
				for i := range app.Spec.Workers {
					if app.Spec.Workers[i].InstanceCount > 0 {
						app.Spec.Workers[i].InstanceCount = 0
						needsUpdate = true
					}
				}

				if needsUpdate {
					_, _, err = client.Apps.Update(ctx, appID, &godo.AppUpdateRequest{Spec: app.Spec})
					assert.NilError(t, err)
					// Wait for deployment
					_ = waitForDeployment(ctx, t, client, appID, 3*time.Minute)
				}
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(t)

			err := provider.InstanceStop(ctx, appID)
			if tt.wantErr {
				assert.Assert(t, err != nil)
			} else {
				assert.NilError(t, err)

				// Verify app was stopped
				app, _, err := client.Apps.Get(ctx, appID)
				assert.NilError(t, err)

				totalInstances := 0
				for _, service := range app.Spec.Services {
					totalInstances += int(service.InstanceCount)
				}
				for _, worker := range app.Spec.Workers {
					totalInstances += int(worker.InstanceCount)
				}

				assert.Equal(t, 0, totalInstances, "app should have 0 instances after stop")
			}
		})
	}
}
