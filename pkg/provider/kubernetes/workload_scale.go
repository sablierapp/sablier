package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	autoscalingv1 "k8s.io/api/autoscaling/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// scaleClient is the part of the typed Deployment and StatefulSet clients that
// scale uses. T is the workload type Patch decodes its response into.
type scaleClient[T any] interface {
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (T, error)
	// GetScale and UpdateScale back the deprecated GET + UPDATE fallback only.
	GetScale(ctx context.Context, name string, options metav1.GetOptions) (*autoscalingv1.Scale, error)
	UpdateScale(ctx context.Context, name string, scale *autoscalingv1.Scale, opts metav1.UpdateOptions) (*autoscalingv1.Scale, error)
}

func (p *Provider) scale(ctx context.Context, config ParsedName, replicas int32) error {
	switch config.Kind {
	case "deployment":
		return scaleWorkload(ctx, p, p.Client.AppsV1().Deployments(config.Namespace), config, replicas)
	case "statefulset":
		return scaleWorkload(ctx, p, p.Client.AppsV1().StatefulSets(config.Namespace), config, replicas)
	default:
		return fmt.Errorf("unsupported kind \"%s\" must be one of \"deployment\", \"statefulset\"", config.Kind)
	}
}

func scaleWorkload[T any](ctx context.Context, p *Provider, c scaleClient[T], config ParsedName, replicas int32) error {
	// A merge patch on the scale subresource carries no resourceVersion, so it
	// cannot fail with a 409 Conflict when the workload is written concurrently
	// (e.g. the controller updating .status right after a previous scale).
	// Sablier sets an absolute replica count, so it has nothing to lock on.
	patch, err := json.Marshal(map[string]any{"spec": map[string]any{"replicas": replicas}})
	if err != nil {
		return err
	}

	// The typed clients decode the returned Scale into a Deployment or
	// StatefulSet, which leaves it empty; only the error is used.
	_, err = c.Patch(ctx, config.Name, types.MergePatchType, patch, metav1.PatchOptions{}, "scale")
	if !apierrors.IsForbidden(err) {
		return err
	}

	// Deprecated: RBAC written for Sablier <= 1.19 grants only get and update on
	// the scale subresource. Keep it working with the previous GET + UPDATE,
	// which can fail with a 409 Conflict, until the next major release.
	p.warnScaleFallback(ctx, config.Kind)
	s, fallbackErr := c.GetScale(ctx, config.Name, metav1.GetOptions{})
	if fallbackErr == nil {
		s.Spec.Replicas = replicas
		_, fallbackErr = c.UpdateScale(ctx, config.Name, s, metav1.UpdateOptions{})
	}
	if apierrors.IsForbidden(fallbackErr) {
		// get/update are not granted either: report the missing patch verb.
		return err
	}
	return fallbackErr
}

// warnScaleFallback logs, once per kind, that scaling fell back to the
// deprecated GET + UPDATE because the patch verb is not granted.
func (p *Provider) warnScaleFallback(ctx context.Context, kind string) {
	if _, warned := p.scaleFallbackWarned.LoadOrStore(kind, struct{}{}); warned {
		return
	}
	resource := kind + "s/scale"
	p.l.WarnContext(ctx, "the patch verb is not granted on "+resource+", falling back to get and update. "+
		"This fallback is deprecated and will be removed in the next major release, and it can fail when the "+
		"workload is written concurrently. Grant patch on "+resource+" in Sablier's ClusterRole.",
		slog.String("resource", resource))
}
