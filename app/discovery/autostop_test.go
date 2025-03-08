package discovery_test

import (
	"errors"
	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/app/discovery"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/provider/providertest"
	"github.com/sablierapp/sablier/pkg/sablier"
	"github.com/sablierapp/sablier/pkg/store/inmemory"
	gomock "go.uber.org/mock/gomock"
	"gotest.tools/v3/assert"
	"testing"
	"time"
)

func TestStopAllUnregisteredInstances(t *testing.T) {
	ctrl := gomock.NewController(t)
	p := providertest.NewMockProvider(ctrl)

	ctx := t.Context()

	// Define instances and registered instances
	instances := []sablier.InstanceConfiguration{
		{Name: "instance1"},
		{Name: "instance2"},
		{Name: "instance3"},
	}
	store := inmemory.NewInMemory()
	err := store.Put(ctx, sablier.InstanceInfo{Name: "instance1"}, time.Minute)
	assert.NilError(t, err)

	// Set up expectations for InstanceList
	p.EXPECT().InstanceList(ctx, provider.InstanceListOptions{
		All:    false,
		Labels: []string{discovery.LabelEnable},
	}).Return(instances, nil)

	// Set up expectations for InstanceStop
	p.EXPECT().InstanceStop(ctx, "instance2").Return(nil)
	p.EXPECT().InstanceStop(ctx, "instance3").Return(nil)

	// Call the function under test
	err = discovery.StopAllUnregisteredInstances(ctx, p, store, slogt.New(t))
	assert.NilError(t, err)
}

func TestStopAllUnregisteredInstances_WithError(t *testing.T) {
	ctrl := gomock.NewController(t)
	p := providertest.NewMockProvider(ctrl)

	ctx := t.Context()

	// Define instances and registered instances
	instances := []sablier.InstanceConfiguration{
		{Name: "instance1"},
		{Name: "instance2"},
		{Name: "instance3"},
	}
	store := inmemory.NewInMemory()
	err := store.Put(ctx, sablier.InstanceInfo{Name: "instance1"}, time.Minute)
	assert.NilError(t, err)

	// Set up expectations for InstanceList
	p.EXPECT().InstanceList(ctx, provider.InstanceListOptions{
		All:    false,
		Labels: []string{discovery.LabelEnable},
	}).Return(instances, nil)

	// Set up expectations for InstanceStop with error
	p.EXPECT().InstanceStop(ctx, "instance2").Return(errors.New("stop error"))
	p.EXPECT().InstanceStop(ctx, "instance3").Return(nil)

	// Call the function under test
	err = discovery.StopAllUnregisteredInstances(ctx, p, store, slogt.New(t))
	assert.Error(t, err, "stop error")
}
