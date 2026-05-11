package dockerswarm_test

import (
	"context"
	"testing"
	"time"

	"github.com/docker/docker/api/types/swarm"
	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider/dockerswarm"
	"gotest.tools/v3/assert"
)

func TestDockerSwarmProvider_NotifyInstanceStopped(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	dind := setupDinD(t)

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	p, err := dockerswarm.New(ctx, dind.client, slogt.New(t), true)
	assert.NilError(t, err)

	waitC := make(chan string)
	p.NotifyInstanceStopped(ctx, waitC)
	<-time.After(time.Second)

	t.Run("labeled service scale to zero sends notification", func(t *testing.T) {
		c, err := dind.CreateMimic(ctx, MimicOptions{Labels: map[string]string{"sablier.enable": "true"}})
		assert.NilError(t, err)

		service, _, err := dind.client.ServiceInspectWithRaw(ctx, c.ID, swarm.ServiceInspectOptions{})
		assert.NilError(t, err)

		replicas := uint64(0)
		service.Spec.Mode.Replicated.Replicas = &replicas

		_, err = p.Client.ServiceUpdate(ctx, service.ID, service.Version, service.Spec, swarm.ServiceUpdateOptions{})
		assert.NilError(t, err)

		name := waitForInstanceStopped(t, waitC)

		// Docker container name is prefixed with a slash, but we don't use it
		assert.Equal(t, name, service.Spec.Name)
	})

	t.Run("labeled service remove sends notification", func(t *testing.T) {
		c, err := dind.CreateMimic(ctx, MimicOptions{Labels: map[string]string{"sablier.enable": "true"}})
		assert.NilError(t, err)

		service, _, err := dind.client.ServiceInspectWithRaw(ctx, c.ID, swarm.ServiceInspectOptions{})
		assert.NilError(t, err)

		err = p.Client.ServiceRemove(ctx, service.ID)
		assert.NilError(t, err)

		name := waitForInstanceStopped(t, waitC)

		// Docker container name is prefixed with a slash, but we don't use it
		assert.Equal(t, name, service.Spec.Name)
	})

	t.Run("unlabeled service scale to zero is ignored", func(t *testing.T) {
		unlabeled, err := dind.CreateMimic(ctx, MimicOptions{})
		assert.NilError(t, err)

		service, _, err := dind.client.ServiceInspectWithRaw(ctx, unlabeled.ID, swarm.ServiceInspectOptions{})
		assert.NilError(t, err)

		replicas := uint64(0)
		service.Spec.Mode.Replicated.Replicas = &replicas

		_, err = p.Client.ServiceUpdate(ctx, service.ID, service.Version, service.Spec, swarm.ServiceUpdateOptions{})
		assert.NilError(t, err)

		assertNoInstanceStopped(t, waitC)
	})

	t.Run("unlabeled service remove is ignored", func(t *testing.T) {
		unlabeled, err := dind.CreateMimic(ctx, MimicOptions{})
		assert.NilError(t, err)

		service, _, err := dind.client.ServiceInspectWithRaw(ctx, unlabeled.ID, swarm.ServiceInspectOptions{})
		assert.NilError(t, err)

		err = p.Client.ServiceRemove(ctx, service.ID)
		assert.NilError(t, err)

		assertNoInstanceStopped(t, waitC)
	})
}

func TestDockerSwarmProvider_NotifyInstanceStopped_UnlabeledWhenIgnoreDisabled(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	dind := setupDinD(t)

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	p, err := dockerswarm.New(ctx, dind.client, slogt.New(t), false)
	assert.NilError(t, err)

	waitC := make(chan string)
	p.NotifyInstanceStopped(ctx, waitC)
	<-time.After(time.Second)

	t.Run("unlabeled service scale to zero sends notification", func(t *testing.T) {
		unlabeled, err := dind.CreateMimic(ctx, MimicOptions{})
		assert.NilError(t, err)

		service, _, err := dind.client.ServiceInspectWithRaw(ctx, unlabeled.ID, swarm.ServiceInspectOptions{})
		assert.NilError(t, err)

		replicas := uint64(0)
		service.Spec.Mode.Replicated.Replicas = &replicas

		_, err = p.Client.ServiceUpdate(ctx, service.ID, service.Version, service.Spec, swarm.ServiceUpdateOptions{})
		assert.NilError(t, err)

		name := waitForInstanceStopped(t, waitC)

		assert.Equal(t, name, service.Spec.Name)
	})

	t.Run("unlabeled service remove sends notification", func(t *testing.T) {
		unlabeled, err := dind.CreateMimic(ctx, MimicOptions{})
		assert.NilError(t, err)

		service, _, err := dind.client.ServiceInspectWithRaw(ctx, unlabeled.ID, swarm.ServiceInspectOptions{})
		assert.NilError(t, err)

		err = p.Client.ServiceRemove(ctx, service.ID)
		assert.NilError(t, err)

		name := waitForInstanceStopped(t, waitC)

		assert.Equal(t, name, service.Spec.Name)
	})
}

func waitForInstanceStopped(t *testing.T, waitC <-chan string) string {
	t.Helper()

	select {
	case name, ok := <-waitC:
		assert.Assert(t, ok, "event stream closed before receiving a stopped instance")
		return name
	case <-time.After(30 * time.Second):
		t.Fatal("timed out waiting for stopped instance notification")
		return ""
	}
}

func assertNoInstanceStopped(t *testing.T, waitC <-chan string) {
	t.Helper()

	select {
	case name, ok := <-waitC:
		assert.Assert(t, ok, "event stream closed while checking for ignored stopped instance")
		t.Fatalf("unexpected stopped instance notification for %q", name)
	case <-time.After(time.Second):
	}
}
