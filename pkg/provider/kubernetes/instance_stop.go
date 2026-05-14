package kubernetes

import (
	"context"
	"log/slog"

	"github.com/sablierapp/sablier/pkg/sablier"
)

func (p *Provider) InstanceStop(ctx context.Context, name string) error {
	parsed, err := ParseName(name, ParseOptions{Delimiter: p.delimiter})
	if err != nil {
		return err
	}

	labels, err := p.getWorkloadLabels(ctx, parsed)
	if err != nil {
		return err
	}

	sc := sablier.ScaleConfigFromLabels(labels)
	if sc != nil && (sc.Idle.CPU != "" || sc.Idle.Memory != "") {
		p.l.DebugContext(ctx, "applying idle resources (scale mode)",
			slog.String("name", name),
			slog.String("cpu", sc.Idle.CPU),
			slog.String("memory", sc.Idle.Memory),
		)
		return p.scaleResources(ctx, parsed, sc.Idle.CPU, sc.Idle.Memory)
	}

	return p.scale(ctx, parsed, 0)
}
