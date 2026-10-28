package kubernetes

// Unit tests for scale's merge patch on the scale subresource.
// These run without a real Kubernetes cluster.

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"gotest.tools/v3/assert"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
	forbidden := apierrors.NewForbidden(schema.GroupResource{Group: appsv1.GroupName, Resource: "deployments/scale"}, "hub", errors.New("denied"))
	p, patches, _ := newScaleTestProvider("deployments", forbidden)

	err := p.scale(context.Background(), ParsedName{Kind: "deployment", Namespace: "ns", Name: "hub"}, 1)
	assert.Assert(t, apierrors.IsForbidden(err), "expected forbidden error, got %v", err)
	assert.Equal(t, len(*patches), 1)
}

func TestScale_UnsupportedKind(t *testing.T) {
	p, patches, _ := newScaleTestProvider("deployments", nil)

	err := p.scale(context.Background(), ParsedName{Kind: "daemonset", Namespace: "ns", Name: "hub"}, 1)
	assert.ErrorContains(t, err, `unsupported kind "daemonset"`)
	assert.Equal(t, len(*patches), 0)
}
