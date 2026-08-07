package kubernetes

import (
	"context"
	"encoding/json"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

// redisOperatorGVR is the GroupVersionResource for OT-CONTAINER-KIT standalone
// Redis custom resources.
var redisOperatorGVR = schema.GroupVersionResource{
	Group:    "redis.redis.opstreelabs.in",
	Version:  "v1beta2",
	Resource: "redis",
}

const redisOperatorSkipReconcileAnnotation = "redis.opstreelabs.in/skip-reconcile"

// redisOperatorOwner returns the name of the Redis CR that controls the
// StatefulSet, if the StatefulSet was created by the OT-CONTAINER-KIT
// redis-operator. The check matches on API group and Kind only, not on the
// specific version, so it remains correct when the operator promotes from
// v1beta2 to v1 or later. Returns ("", false) for any other StatefulSet.
func redisOperatorOwner(ss *appsv1.StatefulSet) (name string, ok bool) {
	for _, ref := range ss.OwnerReferences {
		if ref.Controller != nil && *ref.Controller &&
			ref.Kind == "Redis" &&
			apiVersionGroup(ref.APIVersion) == redisOperatorGVR.Group {
			return ref.Name, true
		}
	}
	return "", false
}

// apiVersionGroup returns the group portion of a "group/version" APIVersion
// string (e.g. "apps" from "apps/v1"). Returns the full string unchanged for
// core resources that have no group prefix.
func apiVersionGroup(apiVersion string) string {
	if i := strings.Index(apiVersion, "/"); i >= 0 {
		return apiVersion[:i]
	}
	return apiVersion
}

// setRedisOperatorSkipReconcile sets or removes the skip-reconcile annotation
// on the Redis CR that owns ss, if any. When skip is true the redis-operator
// pauses its reconciliation loop, allowing Sablier to scale the StatefulSet to
// zero without the operator immediately restoring the replica count. Errors are
// logged but not returned — annotation failure is non-fatal.
func (p *Provider) setRedisOperatorSkipReconcile(ctx context.Context, ss *appsv1.StatefulSet, skip bool) {
	if p.dynamic == nil {
		return
	}
	owner, ok := redisOperatorOwner(ss)
	if !ok {
		return
	}

	var value any = "true"
	if !skip {
		value = nil // JSON merge-patch null removes the key
	}

	patch, err := json.Marshal(map[string]any{
		"metadata": map[string]any{
			"annotations": map[string]any{
				redisOperatorSkipReconcileAnnotation: value,
			},
		},
	})
	if err != nil {
		p.l.WarnContext(ctx, "cannot marshal redis skip-reconcile patch", "error", err)
		return
	}

	_, err = p.dynamic.Resource(redisOperatorGVR).Namespace(ss.Namespace).Patch(
		ctx, owner, types.MergePatchType, patch, metav1.PatchOptions{},
	)
	if err != nil {
		p.l.WarnContext(ctx, "cannot set skip-reconcile on redis owner",
			"redis", owner, "namespace", ss.Namespace, "skip", skip, "error", err)
	}
}
