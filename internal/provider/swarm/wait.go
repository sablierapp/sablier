package swarm

import (
	"context"
	"time"

	"github.com/acouvreur/sablier/internal/provider"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
)

type WaitOptions struct {
	Interval time.Duration
}

// This file is there because the current event for swarm lack of service ready etc.
func (client *Client) Wait(ctx context.Context, name string, opts WaitOptions, in <-chan provider.Message) error {
	messages := make(chan provider.Message)
	closed := make(chan error, 1)

	started := make(chan struct{})
	ticker := time.NewTicker(opts.Interval)
	go func() {
		defer close(closed)
		service, _, err := client.Client.ServiceInspectWithRaw(ctx, name, types.ServiceInspectOptions{})
		service.UpdateStatus.State == swarm.UpdateStateCompleted

		close(started)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				service, _, err := client.Client.ServiceInspectWithRaw(ctx, name, types.ServiceInspectOptions{})

			}
		}
	}()
	<-started

}
