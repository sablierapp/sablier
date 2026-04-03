package kubernetes

import (
	"context"
	"fmt"
	"github.com/sablierapp/sablier/pkg/sablier"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (p *Provider) ensureManaged(ctx context.Context, parsed ParsedName) error {
	var labels map[string]string

	switch parsed.Kind {
	case "deployment":
		d, err := p.Client.AppsV1().Deployments(parsed.Namespace).Get(ctx, parsed.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		labels = d.Labels
	case "statefulset":
		ss, err := p.Client.AppsV1().StatefulSets(parsed.Namespace).Get(ctx, parsed.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		labels = ss.Labels
	default:
		return fmt.Errorf("unsupported kind %q", parsed.Kind)
	}

	if labels["sablier.enable"] != "true" {
		return sablier.ErrInstanceNotManaged{Name: parsed.Original}
	}
	return nil
}

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
