package discovery_test

import (
	"context"
	"errors"
	"github.com/sablierapp/sablier/app/discovery"
	"github.com/sablierapp/sablier/app/instance"
	"github.com/sablierapp/sablier/app/providers"
	"github.com/sablierapp/sablier/app/providers/mock"
	"github.com/sablierapp/sablier/app/types"
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
	mockProvider.On("InstanceList", ctx, providers.InstanceListOptions{
		All:    false,
		Labels: []string{discovery.LabelEnable},
	}).Return(instances, nil)

	// Set up expectations for Stop
	mockProvider.On("Stop", ctx, "instance2").Return(nil)
	mockProvider.On("Stop", ctx, "instance3").Return(nil)

	// Call the function under test
	err = discovery.StopAllUnregisteredInstances(ctx, mockProvider, store)
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
	mockProvider.On("InstanceList", ctx, providers.InstanceListOptions{
		All:    false,
		Labels: []string{discovery.LabelEnable},
	}).Return(instances, nil)

	// Set up expectations for Stop with error
	mockProvider.On("Stop", ctx, "instance2").Return(errors.New("stop error"))
	mockProvider.On("Stop", ctx, "instance3").Return(nil)

	// Call the function under test
	err = discovery.StopAllUnregisteredInstances(ctx, mockProvider, store)
	assert.Error(t, err, "stop error")

	// Check expectations
	mockProvider.AssertExpectations(t)
}
