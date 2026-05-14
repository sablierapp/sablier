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
	if sc != nil && (sc.Active.CPU != "" || sc.Active.Memory != "") {
		p.l.DebugContext(ctx, "applying active resources (scale mode)",
			slog.String("name", name),
			slog.String("cpu", sc.Active.CPU),
			slog.String("memory", sc.Active.Memory),
		)
		return p.scaleResources(ctx, parsed, sc.Active.CPU, sc.Active.Memory)
	}

	return p.scale(ctx, parsed, parsed.Replicas)
}
