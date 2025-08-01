package nomad

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/hashicorp/nomad/api"
)

func (p *Provider) JobStart(ctx context.Context, name string) error {
	// TODO: InstanceStart should block until the job is ready.
	config, err := p.convertName(name)
	if err != nil {
		return fmt.Errorf("cannot convert name %s: %w", name, err)
	}

	p.l.DebugContext(ctx, "starting job", "name", name)
	_, _, err = p.Client.Jobs().Scale(
		config.Job, config.Group, &config.Replicas,
		fmt.Sprintf("Automatically scaled to %d from Sablier", config.Replicas),
		false,
		make(map[string]interface{}),
		&api.WriteOptions{})
	if err != nil {
		p.l.ErrorContext(ctx, "cannot start job", slog.String("name", name), slog.Any("error", err))
		return fmt.Errorf("cannot start job %s: %w", name, err)
	}
	return nil
}
