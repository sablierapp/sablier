package kubernetes

// Unit tests for scale's merge patch on the scale subresource and its
// deprecated GET + UPDATE fallback.
// These run without a real Kubernetes cluster.

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// newScaleTestProvider returns a provider whose fake clientset records every
// patch of the scale subresource of resource and answers it with respond.
func newScaleTestProvider(resource string, respond error) (*Provider, *[]k8stesting.PatchAction, *k8sfake.Clientset) {
	patches := &[]k8stesting.PatchAction{}
	client := k8sfake.NewSimpleClientset()
	client.PrependReactor("patch", resource, func(action k8stesting.Action) (bool, runtime.Object, error) {
		if action.GetSubresource() != "scale" {
			return false, nil, nil
		}
		*patches = append(*patches, action.(k8stesting.PatchAction))
		return true, nil, respond
	})
	return &Provider{Client: client, delimiter: "_", l: slog.Default()}, patches, client
}

func TestScale_MergePatchesScaleSubresource(t *testing.T) {
	for _, tc := range []struct {
		kind, resource string
		replicas       int32
		body           string
	}{
		{"deployment", "deployments", 1, `{"spec":{"replicas":1}}`},
		{"deployment", "deployments", 0, `{"spec":{"replicas":0}}`},
		{"statefulset", "statefulsets", 3, `{"spec":{"replicas":3}}`},
	} {
		t.Run(tc.kind+"/"+tc.body, func(t *testing.T) {
			p, patches, client := newScaleTestProvider(tc.resource, nil)

			err := p.scale(context.Background(), ParsedName{Kind: tc.kind, Namespace: "ns", Name: "hub"}, tc.replicas)
			assert.NilError(t, err)
			assert.Equal(t, len(*patches), 1)
			patch := (*patches)[0]
			assert.Equal(t, patch.GetNamespace(), "ns")
			assert.Equal(t, patch.GetName(), "hub")
			assert.Equal(t, patch.GetPatchType(), types.MergePatchType)
			// No resourceVersion precondition: a concurrent write cannot make it conflict.
			assert.Equal(t, string(patch.GetPatch()), tc.body)
			// A single request: no read-modify-write.
			assert.Equal(t, len(client.Actions()), 1)
		})
	}
}

func TestScale_ReturnsPatchError(t *testing.T) {
	notFound := apierrors.NewNotFound(schema.GroupResource{Group: appsv1.GroupName, Resource: "deployments"}, "hub")
	p, patches, client := newScaleTestProvider("deployments", notFound)

	err := p.scale(context.Background(), ParsedName{Kind: "deployment", Namespace: "ns", Name: "hub"}, 1)
	assert.Assert(t, apierrors.IsNotFound(err), "expected not found error, got %v", err)
	assert.Equal(t, len(*patches), 1)
	// Only a Forbidden patch falls back to GET + UPDATE.
	assert.Equal(t, len(client.Actions()), 1)
}

func forbiddenScale(resource string) error {
	return apierrors.NewForbidden(schema.GroupResource{Group: appsv1.GroupName, Resource: resource + "/scale"}, "hub", errors.New("denied"))
}

// fallbackReactors answers GET and UPDATE on the scale subresource of resource:
// GET returns a Scale with replicas 0, UPDATE records the replicas it was sent.
// getErr/updateErr, when set, are returned instead.
func fallbackReactors(client *k8sfake.Clientset, resource string, getErr, updateErr error) *[]int32 {
	updated := &[]int32{}
	client.PrependReactor("get", resource, func(action k8stesting.Action) (bool, runtime.Object, error) {
		if action.GetSubresource() != "scale" {
			return false, nil, nil
		}
		if getErr != nil {
			return true, nil, getErr
		}
		return true, &autoscalingv1.Scale{ObjectMeta: metav1.ObjectMeta{Name: "hub", Namespace: "ns", ResourceVersion: "7"}}, nil
	})
	client.PrependReactor("update", resource, func(action k8stesting.Action) (bool, runtime.Object, error) {
		if action.GetSubresource() != "scale" {
			return false, nil, nil
		}
		if updateErr != nil {
			return true, nil, updateErr
		}
		s := action.(k8stesting.UpdateAction).GetObject().(*autoscalingv1.Scale)
		*updated = append(*updated, s.Spec.Replicas)
		return true, s, nil
	})
	return updated
}

// RBAC written for Sablier <= 1.19 grants only get and update on the scale
// subresource: a Forbidden patch falls back to the previous GET + UPDATE.
func TestScale_ForbiddenPatch_FallsBackToGetAndUpdate(t *testing.T) {
	for _, tc := range []struct{ kind, resource string }{
		{"deployment", "deployments"},
		{"statefulset", "statefulsets"},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			p, patches, client := newScaleTestProvider(tc.resource, forbiddenScale(tc.resource))
			updated := fallbackReactors(client, tc.resource, nil, nil)

			err := p.scale(context.Background(), ParsedName{Kind: tc.kind, Namespace: "ns", Name: "hub"}, 2)
			assert.NilError(t, err)
			assert.Equal(t, len(*patches), 1)
			assert.DeepEqual(t, *updated, []int32{2})

			var verbs []string
			for _, a := range client.Actions() {
				verbs = append(verbs, a.GetVerb()+" "+a.GetSubresource())
			}
			assert.DeepEqual(t, verbs, []string{"patch scale", "get scale", "update scale"})
		})
	}
}

// When get/update are not granted either, the error names the missing patch.
func TestScale_ForbiddenPatchAndFallback_ReturnsPatchError(t *testing.T) {
	patchErr := forbiddenScale("deployments")
	p, _, client := newScaleTestProvider("deployments", patchErr)
	fallbackReactors(client, "deployments", forbiddenScale("deployments"), nil)

	err := p.scale(context.Background(), ParsedName{Kind: "deployment", Namespace: "ns", Name: "hub"}, 1)
	assert.Equal(t, err, patchErr)
}

// Other fallback failures, such as an update conflict, are returned as they are.
func TestScale_FallbackError_IsReturned(t *testing.T) {
	conflict := apierrors.NewConflict(schema.GroupResource{Group: appsv1.GroupName, Resource: "deployments"}, "hub", errors.New("modified"))
	p, _, client := newScaleTestProvider("deployments", forbiddenScale("deployments"))
	fallbackReactors(client, "deployments", nil, conflict)

	err := p.scale(context.Background(), ParsedName{Kind: "deployment", Namespace: "ns", Name: "hub"}, 1)
	assert.Assert(t, apierrors.IsConflict(err), "expected conflict error, got %v", err)
}

// The fallback runs on every start and stop, so its deprecation warning is
// logged once per kind.
func TestScale_FallbackWarnsOncePerKind(t *testing.T) {
	var logs bytes.Buffer
	client := k8sfake.NewSimpleClientset()
	for _, resource := range []string{"deployments", "statefulsets"} {
		client.PrependReactor("patch", resource, func(action k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, forbiddenScale(resource)
		})
		fallbackReactors(client, resource, nil, nil)
	}
	p := &Provider{Client: client, delimiter: "_", l: slog.New(slog.NewTextHandler(&logs, nil))}

	for range 3 {
		for _, kind := range []string{"deployment", "statefulset"} {
			assert.NilError(t, p.scale(context.Background(), ParsedName{Kind: kind, Namespace: "ns", Name: "hub"}, 1))
		}
	}
	out := logs.String()
	assert.Equal(t, strings.Count(out, "level=WARN"), 2, out)
	assert.Equal(t, strings.Count(out, "resource=deployments/scale"), 1, out)
	assert.Equal(t, strings.Count(out, "resource=statefulsets/scale"), 1, out)
	assert.Assert(t, strings.Contains(out, "deprecated"), out)
}

func TestScale_UnsupportedKind(t *testing.T) {
	p, patches, _ := newScaleTestProvider("deployments", nil)

	err := p.scale(context.Background(), ParsedName{Kind: "daemonset", Namespace: "ns", Name: "hub"}, 1)
	assert.ErrorContains(t, err, `unsupported kind "daemonset"`)
	assert.Equal(t, len(*patches), 0)
}
