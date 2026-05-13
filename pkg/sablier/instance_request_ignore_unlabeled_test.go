package sablier_test

import (
	"testing"
	"time"

	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider/providertest"
	"github.com/sablierapp/sablier/pkg/sablier"
	"github.com/sablierapp/sablier/pkg/store"
	"github.com/sablierapp/sablier/pkg/store/storetest"
	"go.uber.org/mock/gomock"
	"gotest.tools/v3/assert"
)

type ignoreUnlabeledMockProvider struct {
	*providertest.MockProvider
	ignore bool
}

func (p ignoreUnlabeledMockProvider) IgnoreUnlabeled() bool {
	return p.ignore
}

func TestInstanceRequest_NewUnlabeledNotReadyRejectedWhenIgnoreEnabled(t *testing.T) {
	ctrl := gomock.NewController(t)
	provider := ignoreUnlabeledMockProvider{
		MockProvider: providertest.NewMockProvider(ctrl),
		ignore:       true,
	}
	sessions := storetest.NewMockStore(ctrl)
	manager := sablier.New(slogt.New(t), sessions, provider)
	ctx := t.Context()

	stoppedInfo := sablier.InstanceInfo{
		Name:            "nginx",
		CurrentReplicas: 0,
		DesiredReplicas: 1,
		Status:          sablier.InstanceStatusStopped,
	}

	sessions.EXPECT().Get(ctx, "nginx").Return(sablier.InstanceInfo{}, store.ErrKeyNotFound)
	provider.EXPECT().InstanceInspect(ctx, "nginx").Return(stoppedInfo, nil)

	_, err := manager.InstanceRequest(ctx, "nginx", time.Minute)
	assert.ErrorContains(t, err, "instance nginx is not managed by sablier")
}

func TestInstanceRequest_NewUnlabeledReadyRejectedWhenIgnoreEnabled(t *testing.T) {
	ctrl := gomock.NewController(t)
	provider := ignoreUnlabeledMockProvider{
		MockProvider: providertest.NewMockProvider(ctrl),
		ignore:       true,
	}
	sessions := storetest.NewMockStore(ctrl)
	manager := sablier.New(slogt.New(t), sessions, provider)
	ctx := t.Context()

	readyInfo := sablier.InstanceInfo{
		Name:            "nginx",
		CurrentReplicas: 1,
		DesiredReplicas: 1,
		Status:          sablier.InstanceStatusReady,
	}

	sessions.EXPECT().Get(ctx, "nginx").Return(sablier.InstanceInfo{}, store.ErrKeyNotFound)
	provider.EXPECT().InstanceInspect(ctx, "nginx").Return(readyInfo, nil)

	_, err := manager.InstanceRequest(ctx, "nginx", time.Minute)
	assert.ErrorContains(t, err, "instance nginx is not managed by sablier")
}
