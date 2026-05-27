package kubernetes

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// clusterHibernate toggles the CloudNativePG declarative hibernation annotation on a
// Cluster. When hibernate is true the cluster is scaled down and its workload removed
// (PVCs are kept); when false the operator resumes the cluster. This is the
// CloudNativePG equivalent of scaling a Deployment/StatefulSet to or from zero.
func (p *Provider) clusterHibernate(ctx context.Context, config ParsedName, hibernate bool) error {
	if p.dynamic == nil {
		return fmt.Errorf("cnpgcluster support requires a dynamic client, none configured")
	}

	value := cnpgHibernationOff
	if hibernate {
		value = cnpgHibernationOn
	}

	// A merge patch on metadata.annotations adds or overwrites only the hibernation
	// key, leaving any other annotations on the Cluster untouched.
	patch := []byte(fmt.Sprintf(`{"metadata":{"annotations":{%q:%q}}}`, cnpgHibernationAnnotation, value))

	_, err := p.dynamic.Resource(cnpgClusterGVR).Namespace(config.Namespace).Patch(
		ctx, config.Name, types.MergePatchType, patch, metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("cannot set hibernation=%s on cnpg cluster %s/%s: %w", value, config.Namespace, config.Name, err)
	}

	return nil
}
