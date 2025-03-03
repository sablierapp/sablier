package discovery_test

import (
	"context"
	"errors"
	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/app/discovery"
	"github.com/sablierapp/sablier/app/instance"
	"github.com/sablierapp/sablier/app/types"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/provider/mock"
	"github.com/sablierapp/sablier/pkg/store/inmemory"
	"gotest.tools/v3/assert"
	"testing"
	"time"
)

func TestStopAllUnregisteredInstances(t *testing.T) {
	mockProvider := new(mock.ProviderMock)
	ctx := context.TODO()

	// Define instances and registered instances
	instances := []types.Instance{
		{Name: "instance1"},
		{Name: "instance2"},
		{Name: "instance3"},
	}
	store := inmemory.NewInMemory()
	err := store.Put(ctx, instance.State{Name: "instance1"}, time.Minute)
	assert.NilError(t, err)

	// Set up expectations for InstanceList
	mockProvider.On("InstanceList", ctx, provider.InstanceListOptions{
		All:    false,
		Labels: []string{discovery.LabelEnable},
	}).Return(instances, nil)

	// Set up expectations for InstanceStop
	mockProvider.On("InstanceStop", ctx, "instance2").Return(nil)
	mockProvider.On("InstanceStop", ctx, "instance3").Return(nil)

	// Call the function under test
	err = discovery.StopAllUnregisteredInstances(ctx, mockProvider, store, slogt.New(t))
	assert.NilError(t, err)

	// Check expectations
	mockProvider.AssertExpectations(t)
}

func TestStopAllUnregisteredInstances_WithError(t *testing.T) {
	mockProvider := new(mock.ProviderMock)
	ctx := context.TODO()

	// Define instances and registered instances
	instances := []types.Instance{
		{Name: "instance1"},
		{Name: "instance2"},
		{Name: "instance3"},
	}
	store := inmemory.NewInMemory()
	err := store.Put(ctx, instance.State{Name: "instance1"}, time.Minute)
	assert.NilError(t, err)

	// Set up expectations for InstanceList
	mockProvider.On("InstanceList", ctx, provider.InstanceListOptions{
		All:    false,
		Labels: []string{discovery.LabelEnable},
	}).Return(instances, nil)

	// Set up expectations for InstanceStop with error
	mockProvider.On("InstanceStop", ctx, "instance2").Return(errors.New("stop error"))
	mockProvider.On("InstanceStop", ctx, "instance3").Return(nil)

	// Call the function under test
	err = discovery.StopAllUnregisteredInstances(ctx, mockProvider, store, slogt.New(t))
	assert.Error(t, err, "stop error")

	// Check expectations
	mockProvider.AssertExpectations(t)
}
