package sablier_test

import (
	"errors"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
	"github.com/sablierapp/sablier/pkg/store"
	"gotest.tools/v3/assert"
	"testing"
)

func TestStopAllUnregisteredInstances(t *testing.T) {
	s, sessions, p := setupSablier(t)

	ctx := t.Context()

	// Define instances and registered instances
	instances := []sablier.InstanceConfiguration{
		{Name: "instance1"},
		{Name: "instance2"},
	}

	sessions.EXPECT().Get(ctx, "instance1").Return(sablier.InstanceInfo{}, store.ErrKeyNotFound)
	sessions.EXPECT().Get(ctx, "instance2").Return(sablier.InstanceInfo{
		Name:   "instance2",
		Status: sablier.InstanceStatusReady,
	}, nil)

	// Set up expectations for InstanceList
	p.EXPECT().InstanceList(ctx, provider.InstanceListOptions{
		All: false,
	}).Return(instances, nil)

	// Set up expectations for InstanceStop
	p.EXPECT().InstanceStop(ctx, "instance1").Return(nil)

	// Call the function under test
	err := s.StopAllUnregisteredInstances(ctx)
	assert.NilError(t, err)
}

func TestStopAllUnregisteredInstances_WithError(t *testing.T) {
	s, sessions, p := setupSablier(t)
	ctx := t.Context()

	// Define instances and registered instances
	instances := []sablier.InstanceConfiguration{
		{Name: "instance1"},
		{Name: "instance2"},
	}

	sessions.EXPECT().Get(ctx, "instance1").Return(sablier.InstanceInfo{}, store.ErrKeyNotFound)
	sessions.EXPECT().Get(ctx, "instance2").Return(sablier.InstanceInfo{
		Name:   "instance2",
		Status: sablier.InstanceStatusReady,
	}, nil)

	// Set up expectations for InstanceList
	p.EXPECT().InstanceList(ctx, provider.InstanceListOptions{
		All: false,
	}).Return(instances, nil)

	// Set up expectations for InstanceStop with error
	p.EXPECT().InstanceStop(ctx, "instance1").Return(errors.New("stop error"))

	// Call the function under test
	err := s.StopAllUnregisteredInstances(ctx)
	assert.Error(t, err, "stop error")
}
