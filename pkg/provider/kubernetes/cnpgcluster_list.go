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
// or sablier.enable=true across all namespaces. When the dynamic client is unset, the
// CloudNativePG CRD is not installed, or the RBAC denies the list, it returns no
// clusters rather than an error, so the provider keeps working on clusters that don't
// run CloudNativePG.
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
		if apierrors.IsForbidden(err) {
			// The permissions are optional per the docs. Discovery needs no RBAC on the
			// resource, so it separates a missing CRD (the documented, silent skip) from
			// an installed CRD whose rule was left out (worth one warning).
			if p.clusterCRDInstalled(ctx) {
				p.warnClusterListForbidden(ctx, err)
			} else {
				p.l.DebugContext(ctx, "cloudnativepg CRD not installed, skipping cnpg cluster discovery")
			}
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
	parsed := ClusterName(u.GetNamespace(), u.GetName(), ParseOptions{Delimiter: p.delimiter})
	return sablier.InstanceConfigurationFromLabels(parsed.Original, sablierConfig(u.GetLabels(), u.GetAnnotations()))
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
