package kubernetes

// Unit tests for InstanceListOptions.All. They run without a Kubernetes cluster.
// https://github.com/sablierapp/sablier/issues/1095

import (
	"context"
	"sort"
	"testing"

	"github.com/neilotoole/slogt"
	"go.opentelemetry.io/otel"
	"gotest.tools/v3/assert"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
)

func newListTestDeployment(name string, replicas *int32) *appsv1.Deployment {
	return &appsv1.Deployment{
		Name:      name,
		Namespace: "default",
		Labels:    map[string]string{sablier.LabelEnable: "true"},
		Spec:      appsv1.DeploymentSpec{Replicas: replicas},
	}
}

func newListTestStatefulSet(name string, replicas *int32) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		Name:      name,
		Namespace: "default",
		Labels:    map[string]string{sablier.LabelEnable: "true"},
		Spec:      appsv1.StatefulSetSpec{Replicas: replicas},
	}
}

func newListTestProvider(t *testing.T, typed []runtime.Object, clusters []runtime.Object) *Provider {
	t.Helper()
	return &Provider{
		Client: k8sfake.NewSimpleClientset(typed...),
		dynamic: dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
			runtime.NewScheme(),
			map[schema.GroupVersionResource]string{cnpgClusterGVR: "ClusterList"},
			clusters...,
		),
		delimiter: "_",
		l:         slogt.New(t),
		tracer:    otel.Tracer("test"),
	}
}

func instanceNames(instances []sablier.InstanceConfiguration) []string {
	names := make([]string, 0, len(instances))
	for _, i := range instances {
		names = append(names, i.Name)
	}
	sort.Strings(names)
	return names
}

func TestProvider_DeploymentList_All(t *testing.T) {
	t.Parallel()

	zero, one := int32(0), int32(1)
	p := newListTestProvider(t, []runtime.Object{
		newListTestDeployment("running", &one),
		newListTestDeployment("scaled-to-zero", &zero),
		newListTestDeployment("unset-replicas", nil),
	}, nil)

	all, err := p.DeploymentList(context.Background(), provider.InstanceListOptions{All: true})
	assert.NilError(t, err)
	assert.DeepEqual(t, instanceNames(all), []string{
		"deployment_default_running_1",
		"deployment_default_scaled-to-zero_1",
		"deployment_default_unset-replicas_1",
	})

	// A Deployment at 0 replicas is stopped. An unset value defaults to 1.
	running, err := p.DeploymentList(context.Background(), provider.InstanceListOptions{All: false})
	assert.NilError(t, err)
	assert.DeepEqual(t, instanceNames(running), []string{
		"deployment_default_running_1",
		"deployment_default_unset-replicas_1",
	})
}

func TestProvider_StatefulSetList_All(t *testing.T) {
	t.Parallel()

	zero, three := int32(0), int32(3)
	p := newListTestProvider(t, []runtime.Object{
		newListTestStatefulSet("running", &three),
		newListTestStatefulSet("scaled-to-zero", &zero),
		newListTestStatefulSet("unset-replicas", nil),
	}, nil)

	all, err := p.StatefulSetList(context.Background(), provider.InstanceListOptions{All: true})
	assert.NilError(t, err)
	assert.DeepEqual(t, instanceNames(all), []string{
		"statefulset_default_running_1",
		"statefulset_default_scaled-to-zero_1",
		"statefulset_default_unset-replicas_1",
	})

	running, err := p.StatefulSetList(context.Background(), provider.InstanceListOptions{All: false})
	assert.NilError(t, err)
	assert.DeepEqual(t, instanceNames(running), []string{
		"statefulset_default_running_1",
		"statefulset_default_unset-replicas_1",
	})
}

func TestProvider_ClusterList_All(t *testing.T) {
	t.Parallel()

	enabled := map[string]string{sablier.LabelEnable: "true"}
	p := newListTestProvider(t, nil, []runtime.Object{
		newClusterObj("default", "pg-running", enabled, nil, 1, 1),
		newClusterObj("default", "pg-hibernated", enabled,
			map[string]string{cnpgHibernationAnnotation: cnpgHibernationOn}, 1, 0),
		newClusterObj("default", "pg-resumed", enabled,
			map[string]string{cnpgHibernationAnnotation: cnpgHibernationOff}, 1, 1),
	})

	all, err := p.ClusterList(context.Background(), provider.InstanceListOptions{All: true})
	assert.NilError(t, err)
	assert.DeepEqual(t, instanceNames(all), []string{
		"cnpgcluster_default_pg-hibernated_1",
		"cnpgcluster_default_pg-resumed_1",
		"cnpgcluster_default_pg-running_1",
	})

	// A Cluster with hibernation set to on is stopped.
	running, err := p.ClusterList(context.Background(), provider.InstanceListOptions{All: false})
	assert.NilError(t, err)
	assert.DeepEqual(t, instanceNames(running), []string{
		"cnpgcluster_default_pg-resumed_1",
		"cnpgcluster_default_pg-running_1",
	})
}

// StopAllUnregisteredInstances lists with All set to false and stops each
// result. The idle workloads must not be in that list.
func TestProvider_InstanceList_All(t *testing.T) {
	t.Parallel()

	zero, one := int32(0), int32(1)
	enabled := map[string]string{sablier.LabelEnable: "true"}
	p := newListTestProvider(t,
		[]runtime.Object{
			newListTestDeployment("d-running", &one),
			newListTestDeployment("d-idle", &zero),
			newListTestStatefulSet("s-running", &one),
			newListTestStatefulSet("s-idle", &zero),
		},
		[]runtime.Object{
			newClusterObj("default", "pg-running", enabled, nil, 1, 1),
			newClusterObj("default", "pg-idle", enabled,
				map[string]string{cnpgHibernationAnnotation: cnpgHibernationOn}, 1, 0),
		},
	)

	all, err := p.InstanceList(context.Background(), provider.InstanceListOptions{All: true})
	assert.NilError(t, err)
	assert.Equal(t, len(all), 6)

	running, err := p.InstanceList(context.Background(), provider.InstanceListOptions{All: false})
	assert.NilError(t, err)
	assert.DeepEqual(t, instanceNames(running), []string{
		"cnpgcluster_default_pg-running_1",
		"deployment_default_d-running_1",
		"statefulset_default_s-running_1",
	})
}
