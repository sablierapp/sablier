package digitalocean_test

import (
	"context"
	"testing"
	"time"

	"github.com/digitalocean/godo"
	"github.com/sablierapp/sablier/pkg/sablier"
	"gotest.tools/v3/assert"
)

func TestDigitalOceanProvider_InstanceInspect(t *testing.T) {
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
		name       string
		setup      func(t *testing.T)
		wantStatus sablier.InstanceStatus
	}{
		{
			name: "inspect running app",
			setup: func(t *testing.T) {
				// Ensure app has 1 instance
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
					// Wait for deployment to be active
					_ = waitForDeployment(ctx, t, client, appID, 5*time.Minute)
				}
			},
			wantStatus: sablier.InstanceStatusReady,
		},
		{
			name: "inspect stopped app",
			setup: func(t *testing.T) {
				// Ensure app has 0 instances
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
			wantStatus: sablier.InstanceStatusNotReady,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(t)

			info, err := provider.InstanceInspect(ctx, appID)
			assert.NilError(t, err)
			assert.Equal(t, appID, info.Name)
			assert.Equal(t, tt.wantStatus, info.Status)

			t.Logf("App status: %s, Current: %d, Desired: %d",
				info.Status, info.CurrentReplicas, info.DesiredReplicas)
		})
	}
}
