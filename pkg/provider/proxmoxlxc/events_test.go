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

func TestProxmoxLXCProvider_InstanceEvents_Started_ContextCancel(t *testing.T) {
	t.Parallel()

	server := proxmoxlxc.MockServer(t, []string{"pve1"}, []proxmoxlxc.TestContainer{
		{VMID: 100, Name: "web", Status: "running", Tags: "sablier", Node: "pve1"},
	})

	p, err := proxmoxlxc.New(t.Context(), proxmoxlxc.NewTestClient(server.URL), slogt.New(t))
	assert.NilError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	stream := p.InstanceEvents(ctx, provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventStarted},
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

func TestProxmoxLXCProvider_InstanceEvents_AllTypes_ContextCancel(t *testing.T) {
	t.Parallel()

	server := proxmoxlxc.MockServer(t, []string{"pve1"}, []proxmoxlxc.TestContainer{
		{VMID: 100, Name: "web", Status: "running", Tags: "sablier", Node: "pve1"},
	})

	p, err := proxmoxlxc.New(t.Context(), proxmoxlxc.NewTestClient(server.URL), slogt.New(t))
	assert.NilError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	stream := p.InstanceEvents(ctx, provider.InstanceEventsOptions{})

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

// Auto-stop and auto-warm of externally started instances act on started events
// only when they report the instance as enabled. See #1177.
func TestProxmoxLXCProvider_InstanceEvents_StartedIsEnabled(t *testing.T) {
	t.Parallel()

	server, setStatus := proxmoxlxc.MockServerWithControl(t, []string{"pve1"}, []proxmoxlxc.TestContainer{
		{VMID: 100, Name: "web", Status: "stopped", Tags: "sablier;sablier-group-web", Node: "pve1"},
	})
	p, err := proxmoxlxc.NewForTest(t.Context(), proxmoxlxc.NewTestClient(server.URL), slogt.New(t), 20*time.Millisecond)
	assert.NilError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	stream := p.InstanceEvents(ctx, provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventStarted},
	})

	time.Sleep(100 * time.Millisecond) // let the initial scan see the container stopped
	setStatus(100, "running")

	select {
	case ev := <-stream.Events:
		assert.Equal(t, ev.Info.Name, "web")
		assert.Equal(t, ev.Info.IsEnabled(), true)
		assert.DeepEqual(t, ev.Info.Groups, []string{"web"})
	case err := <-stream.Err:
		t.Fatalf("event stream failed: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for the started event")
	}
}
