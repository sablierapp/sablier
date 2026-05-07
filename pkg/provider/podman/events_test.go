package podman_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/moby/moby/client"
	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider/podman"
	"gotest.tools/v3/assert"
)

func TestPodmanProvider_NotifyInstanceStopped(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	pind := sharedPinD
	p, err := podman.New(ctx, pind.client, slogt.New(t))
	assert.NilError(t, err)

	c, err := pind.CreateMimic(ctx, MimicOptions{})
	assert.NilError(t, err)

	inspected, err := pind.client.ContainerInspect(ctx, c.ID, client.ContainerInspectOptions{})
	assert.NilError(t, err)

	_, err = pind.client.ContainerStart(ctx, c.ID, client.ContainerStartOptions{})
	assert.NilError(t, err)

	<-time.After(1 * time.Second)

	waitC := make(chan string)
	go p.NotifyInstanceStopped(ctx, waitC)

	_, err = pind.client.ContainerStop(ctx, c.ID, client.ContainerStopOptions{})
	assert.NilError(t, err)

	name := <-waitC

	// Podman may or may not prefix container names with "/" — compare without the slash.
	assert.Equal(t, name, strings.TrimPrefix(inspected.Container.Name, "/"))
}
