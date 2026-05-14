package dockerswarm

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/sablierapp/sablier/pkg/sablier"
)

func (p *Provider) InstanceStop(ctx context.Context, name string) error {
	service, err := p.getServiceByName(name, ctx)
	if err != nil {
		return fmt.Errorf("cannot get service: %w", err)
	}

	sc := sablier.ScaleConfigFromLabels(service.Spec.Labels)
	p.l.DebugContext(ctx, "stopping service",
		slog.String("name", name),
		slog.Int("replicas", int(sc.Idle.Replicas)),
		slog.String("cpu", sc.Idle.CPU),
		slog.String("memory", sc.Idle.Memory),
	)
	return p.ServiceUpdateScale(ctx, name, uint64(sc.Idle.Replicas), sc.Idle.CPU, sc.Idle.Memory)
}
