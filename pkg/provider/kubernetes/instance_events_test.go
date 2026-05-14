package kubernetes_test

import (
	"context"
	"testing"
	"time"

	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/config"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/provider/kubernetes"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestKubernetesProvider_InstanceEvents(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	kind := sharedKinD
	conf := config.NewProviderConfig().Kubernetes
	conf.QPS = 100
	conf.Burst = 100
	p, err := kubernetes.New(ctx, kind.client, slogt.New(t), conf)
	assert.NilError(t, err)

	stream := p.InstanceEvents(ctx, provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{provider.InstanceEventStopped},
	})

	t.Run("deployment is scaled to 0 replicas", func(t *testing.T) {
		d, err := kind.CreateMimicDeployment(ctx, MimicOptions{})
		assert.NilError(t, err)

		err = WaitForDeploymentReady(ctx, kind.client, d.Namespace, d.Name)
		assert.NilError(t, err)

		s, err := p.Client.AppsV1().Deployments(d.Namespace).GetScale(ctx, d.Name, metav1.GetOptions{})
		assert.NilError(t, err)

		s.Spec.Replicas = 0
		_, err = p.Client.AppsV1().Deployments(d.Namespace).UpdateScale(ctx, d.Name, s, metav1.UpdateOptions{})
		assert.NilError(t, err)

		select {
		case info := <-stream.Events:
			assert.Equal(t, info.Name, kubernetes.DeploymentName(d, kubernetes.ParseOptions{Delimiter: "_"}).Original)
			assert.Equal(t, info.Provider, "kubernetes")
			assert.Assert(t, info.Kubernetes != nil)
		case err := <-stream.Err:
			t.Fatalf("unexpected error: %v", err)
		}
	})
	t.Run("deployment is removed", func(t *testing.T) {
		d, err := kind.CreateMimicDeployment(ctx, MimicOptions{})
		assert.NilError(t, err)

		err = WaitForDeploymentReady(ctx, kind.client, d.Namespace, d.Name)
		assert.NilError(t, err)

		err = p.Client.AppsV1().Deployments(d.Namespace).Delete(ctx, d.Name, metav1.DeleteOptions{})
		assert.NilError(t, err)

		select {
		case info := <-stream.Events:
			assert.Equal(t, info.Name, kubernetes.DeploymentName(d, kubernetes.ParseOptions{Delimiter: "_"}).Original)
			assert.Equal(t, info.Provider, "kubernetes")
			assert.Assert(t, info.Kubernetes != nil)
		case err := <-stream.Err:
			t.Fatalf("unexpected error: %v", err)
		}
	})
	t.Run("statefulSet is scaled to 0 replicas", func(t *testing.T) {
		ss, err := kind.CreateMimicStatefulSet(ctx, MimicOptions{})
		assert.NilError(t, err)

		err = WaitForStatefulSetReady(ctx, kind.client, ss.Namespace, ss.Name)
		assert.NilError(t, err)

		s, err := p.Client.AppsV1().StatefulSets(ss.Namespace).GetScale(ctx, ss.Name, metav1.GetOptions{})
		assert.NilError(t, err)

		s.Spec.Replicas = 0
		_, err = p.Client.AppsV1().StatefulSets(ss.Namespace).UpdateScale(ctx, ss.Name, s, metav1.UpdateOptions{})
		assert.NilError(t, err)

		select {
		case info := <-stream.Events:
			assert.Equal(t, info.Name, kubernetes.StatefulSetName(ss, kubernetes.ParseOptions{Delimiter: "_"}).Original)
			assert.Equal(t, info.Provider, "kubernetes")
			assert.Assert(t, info.Kubernetes != nil)
		case err := <-stream.Err:
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("statefulSet is removed", func(t *testing.T) {
		ss, err := kind.CreateMimicStatefulSet(ctx, MimicOptions{})
		assert.NilError(t, err)

		err = WaitForStatefulSetReady(ctx, kind.client, ss.Namespace, ss.Name)
		assert.NilError(t, err)

		err = p.Client.AppsV1().StatefulSets(ss.Namespace).Delete(ctx, ss.Name, metav1.DeleteOptions{})
		assert.NilError(t, err)

		select {
		case info := <-stream.Events:
			assert.Equal(t, info.Name, kubernetes.StatefulSetName(ss, kubernetes.ParseOptions{Delimiter: "_"}).Original)
			assert.Equal(t, info.Provider, "kubernetes")
			assert.Assert(t, info.Kubernetes != nil)
		case err := <-stream.Err:
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestKubernetesProvider_InstanceEvents_Started(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	kind := sharedKinD
	conf := config.NewProviderConfig().Kubernetes
	conf.QPS = 100
	conf.Burst = 100
	p, err := kubernetes.New(ctx, kind.client, slogt.New(t), conf)
	assert.NilError(t, err)

	t.Run("deployment is scaled from 0 replicas", func(t *testing.T) {
		d, err := kind.CreateMimicDeployment(ctx, MimicOptions{})
		assert.NilError(t, err)

		err = WaitForDeploymentReady(ctx, kind.client, d.Namespace, d.Name)
		assert.NilError(t, err)

		s, err := p.Client.AppsV1().Deployments(d.Namespace).GetScale(ctx, d.Name, metav1.GetOptions{})
		assert.NilError(t, err)

		s.Spec.Replicas = 0
		_, err = p.Client.AppsV1().Deployments(d.Namespace).UpdateScale(ctx, d.Name, s, metav1.UpdateOptions{})
		assert.NilError(t, err)
		err = WaitForDeploymentScale(ctx, kind.client, d.Namespace, d.Name, 0)
		assert.NilError(t, err)

		s, err = p.Client.AppsV1().Deployments(d.Namespace).GetScale(ctx, d.Name, metav1.GetOptions{})
		assert.NilError(t, err)

		stream := p.InstanceEvents(ctx, provider.InstanceEventsOptions{
			Types: []provider.InstanceEventType{provider.InstanceEventStarted},
		})

		s.Spec.Replicas = 1
		_, err = p.Client.AppsV1().Deployments(d.Namespace).UpdateScale(ctx, d.Name, s, metav1.UpdateOptions{})
		assert.NilError(t, err)

		select {
		case info := <-stream.Events:
			assert.Equal(t, info.Name, kubernetes.DeploymentName(d, kubernetes.ParseOptions{Delimiter: "_"}).Original)
			assert.Equal(t, info.Provider, "kubernetes")
			assert.Assert(t, info.Kubernetes != nil)
		case err := <-stream.Err:
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("statefulSet is scaled from 0 replicas", func(t *testing.T) {
		ss, err := kind.CreateMimicStatefulSet(ctx, MimicOptions{})
		assert.NilError(t, err)

		err = WaitForStatefulSetReady(ctx, kind.client, ss.Namespace, ss.Name)
		assert.NilError(t, err)

		s, err := p.Client.AppsV1().StatefulSets(ss.Namespace).GetScale(ctx, ss.Name, metav1.GetOptions{})
		assert.NilError(t, err)

		s.Spec.Replicas = 0
		_, err = p.Client.AppsV1().StatefulSets(ss.Namespace).UpdateScale(ctx, ss.Name, s, metav1.UpdateOptions{})
		assert.NilError(t, err)
		err = WaitForStatefulSetScale(ctx, kind.client, ss.Namespace, ss.Name, 0)
		assert.NilError(t, err)

		s, err = p.Client.AppsV1().StatefulSets(ss.Namespace).GetScale(ctx, ss.Name, metav1.GetOptions{})
		assert.NilError(t, err)

		stream := p.InstanceEvents(ctx, provider.InstanceEventsOptions{
			Types: []provider.InstanceEventType{provider.InstanceEventStarted},
		})

		s.Spec.Replicas = 1
		_, err = p.Client.AppsV1().StatefulSets(ss.Namespace).UpdateScale(ctx, ss.Name, s, metav1.UpdateOptions{})
		assert.NilError(t, err)

		select {
		case info := <-stream.Events:
			assert.Equal(t, info.Name, kubernetes.StatefulSetName(ss, kubernetes.ParseOptions{Delimiter: "_"}).Original)
			assert.Equal(t, info.Provider, "kubernetes")
			assert.Assert(t, info.Kubernetes != nil)
		case err := <-stream.Err:
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
