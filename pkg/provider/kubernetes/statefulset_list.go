package kubernetes

import (
	"context"
	"github.com/sablierapp/sablier/app/discovery"
	"github.com/sablierapp/sablier/app/types"
	"github.com/sablierapp/sablier/pkg/provider"
	v1 "k8s.io/api/apps/v1"
	core_v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
)

func (p *KubernetesProvider) StatefulSetList(ctx context.Context, options provider.InstanceListOptions) ([]types.Instance, error) {
	statefulSets, err := p.Client.AppsV1().StatefulSets(core_v1.NamespaceAll).List(ctx, metav1.ListOptions{
		LabelSelector: strings.Join(options.Labels, ","),
	})

	if err != nil {
		return nil, err
	}

	instances := make([]types.Instance, 0, len(statefulSets.Items))
	for _, ss := range statefulSets.Items {
		instance := p.statefulSetToInstance(&ss)
		instances = append(instances, instance)
	}

	return instances, nil
}

func (p *KubernetesProvider) statefulSetToInstance(ss *v1.StatefulSet) types.Instance {
	var group string

	if _, ok := ss.Labels[discovery.LabelEnable]; ok {
		if g, ok := ss.Labels[discovery.LabelGroup]; ok {
			group = g
		} else {
			group = discovery.LabelGroupDefaultValue
		}
	}

	parsed := StatefulSetName(ss, ParseOptions{Delimiter: p.delimiter})

	return types.Instance{
		Name:  parsed.Original,
		Group: group,
	}
}
