package kubernetes

import (
	"context"
	"log/slog"

	"github.com/sablierapp/sablier/pkg/sablier"
)

func (p *Provider) InstanceStart(ctx context.Context, name string) error {
	parsed, err := ParseName(name, ParseOptions{Delimiter: p.delimiter})
	if err != nil {
		return err
	}

	labels, err := p.getWorkloadLabels(ctx, parsed)
	if err != nil {
		return err
	}

	sc := sablier.ScaleConfigFromLabels(labels)
	if sc.Idle.Replicas >= 1 || sc.Active.Replicas > 1 || sc.Active.CPU != "" || sc.Active.Memory != "" {
		p.l.DebugContext(ctx, "applying active resources (scale mode)",
			slog.String("name", name),
			slog.Int("replicas", int(sc.Active.Replicas)),
			slog.String("cpu", sc.Active.CPU),
			slog.String("memory", sc.Active.Memory),
		)
		if err := p.scale(ctx, parsed, sc.Active.Replicas); err != nil {
			return err
		}
		if sc.Active.CPU != "" || sc.Active.Memory != "" {
			return p.scaleResources(ctx, parsed, sc.Active.CPU, sc.Active.Memory)
		}
		return nil
	}

	return p.scale(ctx, parsed, sc.Active.Replicas)
}
