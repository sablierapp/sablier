package kubernetes

import (
	"context"
	"fmt"

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
//
// Without the RBAC rule the API server answers Forbidden even when the CRD is absent, so
// a Forbidden list is resolved through discovery: an absent CRD is skipped like NotFound,
// while an installed CRD returns the error, so callers keep their previous state instead
// of dropping every CloudNativePG Cluster.
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
		state := cnpgCRDUnknown
		if apierrors.IsForbidden(err) {
			state, _ = p.clusterCRDState(ctx)
		}
		switch {
		case apierrors.IsNotFound(err), state == cnpgCRDAbsent:
			p.l.DebugContext(ctx, "cloudnativepg CRD not installed, skipping cnpg cluster discovery")
			return nil, nil
		case state == cnpgCRDInstalled:
			return nil, fmt.Errorf("cloudnativepg CRD is installed but listing clusters is forbidden: "+
				"grant get, list, watch and patch on clusters.postgresql.cnpg.io in Sablier's ClusterRole "+
				"(clusters are listed in all namespaces, so namespaced Roles are not enough): %w", err)
		default:
			return nil, err
		}
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
