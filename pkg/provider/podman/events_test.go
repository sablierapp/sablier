package podman_test

import (
	"context"
	"github.com/containers/podman/v5/pkg/bindings/containers"
	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider/podman"
	"gotest.tools/v3/assert"
	"testing"
	"time"
)

func TestPodmanProvider_NotifyInstanceStopped(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	pind := setupPinD(t)
	p, err := podman.New(pind.connText, slogt.New(t), true)
	assert.NilError(t, err)

	c, err := pind.CreateMimic(ctx, MimicOptions{Labels: map[string]string{"sablier.enable": "true"}})
	assert.NilError(t, err)

	inspected, err := containers.Inspect(pind.connText, c.ID, nil)
	assert.NilError(t, err)

	err = containers.Start(pind.connText, c.ID, nil)
	assert.NilError(t, err)

	<-time.After(1 * time.Second)

	waitC := make(chan string)
	go p.NotifyInstanceStopped(ctx, waitC)

	err = containers.Stop(pind.connText, c.ID, nil)
	assert.NilError(t, err)

	name := waitForInstanceStopped(t, waitC)

	assert.Equal(t, name, inspected.Name)

	unlabeled, err := pind.CreateMimic(ctx, MimicOptions{})
	assert.NilError(t, err)

	err = containers.Start(pind.connText, unlabeled.ID, nil)
	assert.NilError(t, err)

	err = containers.Stop(pind.connText, unlabeled.ID, nil)
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
