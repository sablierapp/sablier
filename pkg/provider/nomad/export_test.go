package nomad

import (
	"context"
	"log/slog"
	"time"

	"github.com/hashicorp/nomad/api"
)

// NewForTest creates a Provider with a short event stream backoff for testing.
func NewForTest(ctx context.Context, client *api.Client, namespace string, logger *slog.Logger, backoff time.Duration) (*Provider, error) {
	p, err := New(ctx, client, namespace, logger)
	if err != nil {
		return nil, err
	}
	p.backoffMin = backoff
	p.backoffMax = backoff * 4
	return p, nil
}
