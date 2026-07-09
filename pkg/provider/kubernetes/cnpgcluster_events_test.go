package kubernetes

import (
	"context"
	"testing"

	"github.com/neilotoole/slogt"
	"gotest.tools/v3/assert"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestProvider_ClusterCRDInstalled(t *testing.T) {
	t.Parallel()

	cnpgGroupVersion := cnpgClusterGVR.GroupVersion().String()

	t.Run("false when dynamic client is nil", func(t *testing.T) {
		t.Parallel()
		p := &Provider{Client: k8sfake.NewSimpleClientset(), l: slogt.New(t)}
		assert.Equal(t, p.clusterCRDInstalled(context.Background()), false)
	})

	t.Run("false when typed client is nil", func(t *testing.T) {
		t.Parallel()
		p := newFakeCNPGProvider(t)
		assert.Equal(t, p.clusterCRDInstalled(context.Background()), false)
	})

	t.Run("true when the clusters resource is served", func(t *testing.T) {
		t.Parallel()
		client := k8sfake.NewSimpleClientset()
		client.Resources = []*metav1.APIResourceList{{
			GroupVersion: cnpgGroupVersion,
			APIResources: []metav1.APIResource{{Name: "clusters/status"}, {Name: cnpgClusterGVR.Resource}},
		}}
		p := newFakeCNPGProvider(t)
		p.Client = client
		assert.Equal(t, p.clusterCRDInstalled(context.Background()), true)
	})

	t.Run("false when the group version is served without the clusters resource", func(t *testing.T) {
		t.Parallel()
		client := k8sfake.NewSimpleClientset()
		client.Resources = []*metav1.APIResourceList{{
			GroupVersion: cnpgGroupVersion,
			APIResources: []metav1.APIResource{{Name: "clusters/status"}},
		}}
		p := newFakeCNPGProvider(t)
		p.Client = client
		assert.Equal(t, p.clusterCRDInstalled(context.Background()), false)
	})

	t.Run("false when the group version is not served", func(t *testing.T) {
		t.Parallel()
		p := newFakeCNPGProvider(t)
		p.Client = k8sfake.NewSimpleClientset()
		assert.Equal(t, p.clusterCRDInstalled(context.Background()), false)
	})

	t.Run("true on non-NotFound discovery error (fail open)", func(t *testing.T) {
		t.Parallel()
		client := k8sfake.NewSimpleClientset()
		client.PrependReactor("get", "resource", func(k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, apierrors.NewForbidden(schema.GroupResource{}, "", nil)
		})
		p := newFakeCNPGProvider(t)
		p.Client = client
		assert.Equal(t, p.clusterCRDInstalled(context.Background()), true)
	})
}
