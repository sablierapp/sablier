package dockerswarm

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/sablierapp/sablier/pkg/sablier"
)

func (p *Provider) InstanceStart(ctx context.Context, name string) error {
	service, err := p.getServiceByName(name, ctx)
	if err != nil {
		return fmt.Errorf("cannot get service: %w", err)
	}

	sc := sablier.ScaleConfigFromLabels(service.Spec.Labels)
	if sc != nil && sc.Active.Replicas >= 1 {
		p.l.DebugContext(ctx, "applying active resources (scale mode)",
			slog.String("name", name),
			slog.Int("replicas", int(sc.Active.Replicas)),
			slog.String("cpu", sc.Active.CPU),
			slog.String("memory", sc.Active.Memory),
		)
		return p.ServiceUpdateScale(ctx, name, uint64(sc.Active.Replicas), sc.Active.CPU, sc.Active.Memory)
	}

	return p.ServiceUpdateReplicas(ctx, name, uint64(p.desiredReplicas))
}
