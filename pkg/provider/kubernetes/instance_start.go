package kubernetes

import "context"

func (p *Provider) InstanceStart(ctx context.Context, name string) error {
	parsed, err := ParseName(name, ParseOptions{Delimiter: p.delimiter})
	if err != nil {
		return err
	}

	return p.scale(ctx, parsed, parsed.Replicas)
}
