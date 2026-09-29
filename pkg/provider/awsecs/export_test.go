package awsecs

import (
	"context"
	"log/slog"
	"time"
)

// NewForTest creates a Provider with a custom poll interval for testing.
func NewForTest(ctx context.Context, client API, cluster string, logger *slog.Logger, pollInterval time.Duration) (*Provider, error) {
	p, err := New(ctx, client, cluster, logger)
	if err != nil {
		return nil, err
	}
	p.pollInterval = pollInterval
	return p, nil
}
