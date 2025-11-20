package digitalocean_test

import (
	"context"
	"testing"
	"time"

	"github.com/digitalocean/godo"
	"gotest.tools/v3/assert"
)

func TestDigitalOceanProvider_InstanceStart(t *testing.T) {
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
			name: "start app scaled to 0",
			setup: func(t *testing.T) {
				// Ensure app is at 0 instances
				app, _, err := client.Apps.Get(ctx, appID)
				assert.NilError(t, err)

				// Scale to 0 if needed
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
		{
			name: "start app already running",
			setup: func(t *testing.T) {
				// Ensure app is at 1 instance
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(t)

			err := provider.InstanceStart(ctx, appID)
			if tt.wantErr {
				assert.Assert(t, err != nil)
			} else {
				assert.NilError(t, err)

				// Verify app was started
				app, _, err := client.Apps.Get(ctx, appID)
				assert.NilError(t, err)

				hasInstances := false
				for _, service := range app.Spec.Services {
					if service.InstanceCount > 0 {
						hasInstances = true
						break
					}
				}
				for _, worker := range app.Spec.Workers {
					if worker.InstanceCount > 0 {
						hasInstances = true
						break
					}
				}

				assert.Assert(t, hasInstances, "app should have at least one instance after start")
			}
		})
	}
}
