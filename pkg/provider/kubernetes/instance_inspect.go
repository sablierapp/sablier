package kubernetes

import (
	"context"
	"fmt"
	"github.com/sablierapp/sablier/pkg/sablier"
)

func (p *Provider) InstanceInspect(ctx context.Context, name string) (sablier.InstanceInfo, error) {
	parsed, err := ParseName(name, ParseOptions{Delimiter: p.delimiter})
	if err != nil {
		return sablier.InstanceInfo{}, err
	}

	switch parsed.Kind {
	case "deployment":
		return p.DeploymentInspect(ctx, parsed)
	case "statefulset":
		return p.StatefulSetInspect(ctx, parsed)
	default:
		return sablier.InstanceInfo{}, fmt.Errorf("unsupported kind \"%s\" must be one of \"deployment\", \"statefulset\"", parsed.Kind)
	}
}
