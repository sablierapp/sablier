package kubernetes

import (
	"context"
	"fmt"
	"github.com/sablierapp/sablier/app/instance"
)

func (p *KubernetesProvider) InstanceInspect(ctx context.Context, name string) (instance.State, error) {
	parsed, err := ParseName(name, ParseOptions{Delimiter: p.delimiter})
	if err != nil {
		return instance.State{}, err
	}

	switch parsed.Kind {
	case "deployment":
		return p.DeploymentInspect(ctx, parsed)
	case "statefulset":
		return p.StatefulSetInspect(ctx, parsed)
	default:
		return instance.State{}, fmt.Errorf("unsupported kind \"%s\" must be one of \"deployment\", \"statefulset\"", parsed.Kind)
	}
}
