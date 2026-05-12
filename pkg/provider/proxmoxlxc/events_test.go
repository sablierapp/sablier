package proxmoxlxc_test

import (
	"context"
	"testing"
	"time"

	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/provider/proxmoxlxc"
	"gotest.tools/v3/assert"
)

func TestProxmoxLXCProvider_InstanceEvents_ContextCancel(t *testing.T) {
	t.Parallel()

	server := proxmoxlxc.MockServer(t, []string{"pve1"}, []proxmoxlxc.TestContainer{
		{VMID: 100, Name: "web", Status: "running", Tags: "sablier", Node: "pve1"},
	})
	defer server.Close()

	p, err := proxmoxlxc.New(t.Context(), proxmoxlxc.NewTestClient(server.URL), slogt.New(t))
	assert.NilError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	stream := p.InstanceEvents(ctx, provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventStopped},
	})

	// Cancel context and verify both channels are closed
	cancel()

	deadline := time.After(3 * time.Second)
	eventsClosed, errClosed := false, false
	for !eventsClosed || !errClosed {
		select {
		case _, ok := <-stream.Events:
			if !ok {
				eventsClosed = true
			}
		case _, ok := <-stream.Err:
			if !ok {
				errClosed = true
			}
		case <-deadline:
			t.Fatal("timed out waiting for stream channels to close")
		}
	}
}
