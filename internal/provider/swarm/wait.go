package swarm

import (
	"context"
	"errors"
	"time"

	"github.com/acouvreur/sablier/internal/provider"
)

type WaitOptions struct {
	Interval time.Duration
}

// This file is there because the current event for swarm lack of service ready etc.
func (client *Client) Wait(ctx context.Context, name string, opts WaitOptions, in <-chan provider.Message) error {
	// TODO: Implement me
	return errors.New("implement me")
}
