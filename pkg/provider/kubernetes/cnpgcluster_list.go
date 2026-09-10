package kubernetes

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
)

// listClusters returns the CloudNativePG Clusters labelled sablierapp.dev/enable=true
// or sablier.enable=true across all namespaces. When the dynamic client is unset or the
// CloudNativePG CRD is not installed, it returns no clusters rather than an error, so the
// provider keeps working on clusters that don't run CloudNativePG.
func (p *Provider) listClusters(ctx context.Context) ([]unstructured.Unstructured, error) {
	if p.dynamic == nil {
		return nil, nil
	}

	items, err := listEnabled(func(opts metav1.ListOptions) ([]unstructured.Unstructured, error) {
		list, err := p.dynamic.Resource(cnpgClusterGVR).Namespace(metav1.NamespaceAll).List(ctx, opts)
		if err != nil {
			return nil, err
		}
		return list.Items, nil
	})
	if err != nil {
		if apierrors.IsNotFound(err) {
			p.l.DebugContext(ctx, "cloudnativepg CRD not installed, skipping cnpg cluster discovery")
			return nil, nil
		}
		return nil, err
	}

	return items, nil
}

func (p *Provider) ClusterList(ctx context.Context, opts provider.InstanceListOptions) ([]sablier.InstanceConfiguration, error) {
	items, err := p.listClusters(ctx)
	if err != nil {
		return nil, err
	}

	instances := make([]sablier.InstanceConfiguration, 0, len(items))
	for i := range items {
		if !opts.All && !clusterIsRunning(&items[i]) {
			continue
		}
		instances = append(instances, p.clusterToInstance(&items[i]))
	}

	return instances, nil
}

// The provider stops a Cluster when it sets the hibernation annotation to on.
func clusterIsRunning(u *unstructured.Unstructured) bool {
	return u.GetAnnotations()[cnpgHibernationAnnotation] != cnpgHibernationOn
}

func (p *Provider) clusterToInstance(u *unstructured.Unstructured) sablier.InstanceConfiguration {
	config := sablierConfig(u.GetLabels(), u.GetAnnotations())
	enabled := config[sablier.LabelEnable]
	var groups []string
	if enabled == "true" {
		groups = sablier.ParseGroups(config[sablier.LabelGroup])
	}

	parsed := ClusterName(u.GetNamespace(), u.GetName(), ParseOptions{Delimiter: p.delimiter})

	return sablier.InstanceConfiguration{
		Name:    parsed.Original,
		Groups:  groups,
		Enabled: enabled,
	}
}

func (p *Provider) ClusterGroups(ctx context.Context) (map[string][]string, error) {
	items, err := p.listClusters(ctx)
	if err != nil {
		return nil, err
	}

	groups := make(map[string][]string)
	for i := range items {
		u := &items[i]
		parsed := ClusterName(u.GetNamespace(), u.GetName(), ParseOptions{Delimiter: p.delimiter})
		config := sablierConfig(u.GetLabels(), u.GetAnnotations())
		for _, groupName := range sablier.ParseGroups(config[sablier.LabelGroup]) {
			groups[groupName] = append(groups[groupName], parsed.Original)
		}
	}

	return groups, nil
}
