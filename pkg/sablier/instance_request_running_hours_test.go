package sablier_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/sablierapp/sablier/pkg/sablier"
	"go.uber.org/mock/gomock"
	"gotest.tools/v3/assert"
)

func TestInstanceRequest_ExtendsExpirationInsideRunningHours(t *testing.T) {
	manager, sessions, _ := setupSablier(t)
	ctx := t.Context()

	now := time.Now()
	start := now.Add(-time.Minute)
	end := now.Add(3 * time.Minute)
	spec := fmt.Sprintf("%02d:%02d-%02d:%02d", start.Hour(), start.Minute(), end.Hour(), end.Minute())

	state := sablier.InstanceInfo{
		Name:         "nginx",
		Status:       sablier.InstanceStatusReady,
		RunningHours: spec,
	}

	sessions.EXPECT().Get(ctx, "nginx").Return(state, nil)
	sessions.EXPECT().Put(ctx, state, gomock.Any()).DoAndReturn(func(_ context.Context, _ sablier.InstanceInfo, ttl time.Duration) error {
		if ttl <= 10*time.Second {
			t.Fatalf("expected ttl to be extended during running-hours, got %s", ttl)
		}
		return nil
	})

	info, err := manager.InstanceRequest(ctx, "nginx", 10*time.Second)
	assert.NilError(t, err)
	assert.Equal(t, info.Name, "nginx")
}

func TestInstanceRequest_DoesNotExtendExpirationOutsideRunningHours(t *testing.T) {
	manager, sessions, _ := setupSablier(t)
	ctx := t.Context()

	now := time.Now()
	start := now.Add(-4 * time.Hour)
	end := now.Add(-2 * time.Hour)
	spec := fmt.Sprintf("%02d:%02d-%02d:%02d", start.Hour(), start.Minute(), end.Hour(), end.Minute())

	state := sablier.InstanceInfo{
		Name:         "nginx",
		Status:       sablier.InstanceStatusReady,
		RunningHours: spec,
	}

	sessions.EXPECT().Get(ctx, "nginx").Return(state, nil)
	sessions.EXPECT().Put(ctx, state, 20*time.Second).Return(nil)

	info, err := manager.InstanceRequest(ctx, "nginx", 20*time.Second)
	assert.NilError(t, err)
	assert.Equal(t, info.Name, "nginx")
}
