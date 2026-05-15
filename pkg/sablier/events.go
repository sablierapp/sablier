package sablier

import (
	"context"

	"github.com/sablierapp/sablier/pkg/provider"
)

// InstanceEvents proxies to the provider's event stream. Each call creates an
// independent subscription, so multiple concurrent callers each receive all events.
func (s *Sablier) InstanceEvents(ctx context.Context, opts provider.InstanceEventsOptions) InstanceEventStream {
	return s.provider.InstanceEvents(ctx, opts)
}
