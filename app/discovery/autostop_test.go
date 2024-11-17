package discovery_test

import (
	"context"
	"errors"
	"github.com/sablierapp/sablier/app/discovery"
	"github.com/sablierapp/sablier/app/providers/mock"
	"github.com/sablierapp/sablier/app/types"
	"testing"
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
	registered := []string{"instance1"}

	// Set up expectations for List
	mockProvider.On("List", ctx).Return(instances, nil)

	// Set up expectations for Stop
	mockProvider.On("Stop", ctx, "instance2").Return(nil)
	mockProvider.On("Stop", ctx, "instance3").Return(nil)

	// Call the function under test
	err := discovery.StopAllUnregisteredInstances(ctx, mockProvider, registered)
	if err != nil {
		t.Fatalf("Expected no error, but got %v", err)
	}

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
	registered := []string{"instance1"}

	// Set up expectations for List
	mockProvider.On("List", ctx).Return(instances, nil)

	// Set up expectations for Stop with error
	mockProvider.On("Stop", ctx, "instance2").Return(errors.New("stop error"))
	mockProvider.On("Stop", ctx, "instance3").Return(nil)

	// Call the function under test
	err := discovery.StopAllUnregisteredInstances(ctx, mockProvider, registered)
	if err == nil {
		t.Fatalf("Expected error, but got nil")
	}

	// Check expectations
	mockProvider.AssertExpectations(t)
}
