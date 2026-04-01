package proxmoxlxc_test

import (
	"context"
	"testing"
	"time"

	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider/proxmoxlxc"
	"gotest.tools/v3/assert"
)

func TestProxmoxLXCProvider_NotifyInstanceStopped_ContextCancel(t *testing.T) {
	t.Parallel()

	server := proxmoxlxc.MockServer(t, []string{"pve1"}, []proxmoxlxc.TestContainer{
		{VMID: 100, Name: "web", Status: "running", Tags: "sablier", Node: "pve1"},
	})
	defer server.Close()

	p, err := proxmoxlxc.New(t.Context(), proxmoxlxc.NewTestClient(server.URL), slogt.New(t))
	assert.NilError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	stoppedCh := make(chan string, 1)
	go p.NotifyInstanceStopped(ctx, stoppedCh)

	// Cancel context and verify channel is closed
	cancel()
	select {
	case _, ok := <-stoppedCh:
		if ok {
			// Got a value, that's unexpected but not fatal — just wait for close
			_, ok = <-stoppedCh
		}
		assert.Assert(t, !ok, "channel should be closed after context cancel")
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for channel close")
	}
}
