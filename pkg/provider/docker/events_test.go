package docker_test

import (
	"context"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider/docker"
	"gotest.tools/v3/assert"
)

func TestDockerClassicProvider_NotifyInstanceStopped(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	dind := setupDinD(t)
	p, err := docker.New(ctx, dind.client, slogt.New(t), "stop", true)
	assert.NilError(t, err)

	c, err := dind.CreateMimic(ctx, MimicOptions{Labels: map[string]string{"sablier.enable": "true"}})
	assert.NilError(t, err)

	inspected, err := dind.client.ContainerInspect(ctx, c.ID)
	assert.NilError(t, err)

	err = dind.client.ContainerStart(ctx, c.ID, container.StartOptions{})
	assert.NilError(t, err)

	<-time.After(1 * time.Second)

	waitC := make(chan string)
	go p.NotifyInstanceStopped(ctx, waitC)

	err = dind.client.ContainerStop(ctx, c.ID, container.StopOptions{})
	assert.NilError(t, err)

	name := waitForInstanceStopped(t, waitC)

	// Docker container name is prefixed with a slash, but we don't use it
	assert.Equal(t, "/"+name, inspected.Name)

	unlabeled, err := dind.CreateMimic(ctx, MimicOptions{})
	assert.NilError(t, err)

	err = dind.client.ContainerStart(ctx, unlabeled.ID, container.StartOptions{})
	assert.NilError(t, err)

	err = dind.client.ContainerStop(ctx, unlabeled.ID, container.StopOptions{})
	assert.NilError(t, err)

	assertNoInstanceStopped(t, waitC)
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
