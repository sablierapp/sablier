package configmap

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/sablierapp/sablier/pkg/sablier"
	storemod "github.com/sablierapp/sablier/pkg/store"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

const (
	namespace = "sablier"
	name      = "sablier-sessions"
)

func newStore(t *testing.T, objects ...runtime.Object) (*Store, *fake.Clientset) {
	t.Helper()
	cli := fake.NewClientset(objects...)
	return New(cli.CoreV1().ConfigMaps(namespace), name, slog.Default()), cli
}

func instance(name string) sablier.InstanceInfo {
	return sablier.InstanceInfo{Name: name, Status: sablier.InstanceStatusReady, CurrentReplicas: 1, DesiredReplicas: 1}
}

// writes counts the create and update calls on ConfigMaps.
func writes(cli *fake.Clientset) int {
	n := 0
	for _, a := range cli.Actions() {
		if a.GetResource().Resource == "configmaps" && (a.GetVerb() == "create" || a.GetVerb() == "update") {
			n++
		}
	}
	return n
}

// reload reads the ConfigMap into a fresh store.
func reload(t *testing.T, cli *fake.Clientset) *Store {
	t.Helper()
	s := New(cli.CoreV1().ConfigMaps(namespace), name, slog.Default())
	assert.NilError(t, s.Load(t.Context()))
	return s
}

func TestLoad_MissingConfigMap(t *testing.T) {
	s, cli := newStore(t)
	assert.NilError(t, s.Load(t.Context()))
	assert.NilError(t, s.Save(t.Context()))
	assert.Equal(t, writes(cli), 0, "nothing changed, nothing to write")
}

func TestLoad_APIErrorIsFatal(t *testing.T) {
	s, cli := newStore(t)
	cli.PrependReactor("get", "configmaps", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: "configmaps"}, name, nil)
	})
	assert.ErrorContains(t, s.Load(t.Context()), "forbidden")
}

func TestLoad_CorruptSnapshotStartsFresh(t *testing.T) {
	s, _ := newStore(t, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		BinaryData: map[string][]byte{DataKey: []byte("not gzip")},
	})
	assert.NilError(t, s.Load(t.Context()))
	_, err := s.Get(t.Context(), "web")
	assert.ErrorIs(t, err, storemod.ErrKeyNotFound)
}

func TestSave_RoundTrip(t *testing.T) {
	ctx := t.Context()
	s, cli := newStore(t)
	assert.NilError(t, s.Load(ctx))
	assert.NilError(t, s.Put(ctx, instance("web"), time.Hour))
	assert.NilError(t, s.Save(ctx))

	cm, err := cli.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Equal(t, cm.Labels["app.kubernetes.io/managed-by"], "sablier")

	info, err := reload(t, cli).Get(ctx, "web")
	assert.NilError(t, err)
	assert.Equal(t, info.Status, sablier.InstanceStatusReady)
}

func TestSave_KeepsExpiry(t *testing.T) {
	ctx := t.Context()
	s, cli := newStore(t)
	assert.NilError(t, s.Load(ctx))
	assert.NilError(t, s.Put(ctx, instance("web"), 10*time.Minute))
	assert.NilError(t, s.Save(ctx))

	var expiresAt time.Time
	_ = reload(t, cli).Range(ctx, func(_ sablier.InstanceInfo, e time.Time) { expiresAt = e })
	remaining := time.Until(expiresAt)
	assert.Assert(t, remaining > 9*time.Minute && remaining <= 10*time.Minute, "remaining %s", remaining)
}

func TestRun_NewSessionWrittenImmediately(t *testing.T) {
	ctx := t.Context()
	s, cli := newStore(t)
	s.refreshInterval = time.Hour
	assert.NilError(t, s.Load(ctx))
	go s.Run(ctx)

	assert.NilError(t, s.Put(ctx, instance("web"), time.Hour))
	waitFor(t, func() bool { return writes(cli) == 1 })
}

func TestRun_RefreshIsPooled(t *testing.T) {
	ctx := t.Context()
	s, cli := newStore(t)
	s.refreshInterval = 200 * time.Millisecond
	assert.NilError(t, s.Load(ctx))
	assert.NilError(t, s.Put(ctx, instance("web"), time.Minute))
	assert.NilError(t, s.Save(ctx))
	go s.Run(ctx)

	for range 10 {
		assert.NilError(t, s.Put(ctx, instance("web"), time.Hour))
	}
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, writes(cli), 1, "a refresh must not be written immediately")

	waitFor(t, func() bool { return writes(cli) == 2 })
	var expiresAt time.Time
	_ = reload(t, cli).Range(ctx, func(_ sablier.InstanceInfo, e time.Time) { expiresAt = e })
	assert.Assert(t, time.Until(expiresAt) > 50*time.Minute, "the pooled write carries the refreshed expiry")
}

func TestRun_BurstIsCoalesced(t *testing.T) {
	ctx := t.Context()
	s, cli := newStore(t)
	s.refreshInterval = time.Hour
	assert.NilError(t, s.Load(ctx))
	for i := range 50 {
		assert.NilError(t, s.Put(ctx, instance("web-"+string(rune('a'+i%26))+string(rune('a'+i/26))), time.Hour))
	}
	go s.Run(ctx)

	waitFor(t, func() bool { return writes(cli) == 1 })
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, writes(cli), 1, "50 queued sessions are one write")
	n := 0
	_ = reload(t, cli).Range(ctx, func(sablier.InstanceInfo, time.Time) { n++ })
	assert.Equal(t, n, 50)
}

func TestDelete(t *testing.T) {
	ctx := t.Context()
	s, cli := newStore(t)
	assert.NilError(t, s.Load(ctx))
	assert.NilError(t, s.Put(ctx, instance("web"), time.Hour))
	assert.NilError(t, s.Save(ctx))

	assert.NilError(t, s.Delete(ctx, "missing"))
	assert.Assert(t, !s.pending.Load(), "deleting a missing session changes nothing")

	assert.NilError(t, s.Delete(ctx, "web"))
	assert.Equal(t, len(s.urgent), 1, "a removed session is written immediately")
	assert.NilError(t, s.Save(ctx))
	_, err := reload(t, cli).Get(ctx, "web")
	assert.ErrorIs(t, err, storemod.ErrKeyNotFound)
}

func TestSave_OverwritesConcurrentChange(t *testing.T) {
	ctx := t.Context()
	s, cli := newStore(t, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, ResourceVersion: "1"},
	})
	assert.NilError(t, s.Load(ctx))

	// Someone else edits the ConfigMap after Sablier read it.
	edited := s.cm.DeepCopy()
	edited.Labels = map[string]string{"owner": "someone"}
	edited.ResourceVersion = "2"
	_, err := cli.CoreV1().ConfigMaps(namespace).Update(ctx, edited, metav1.UpdateOptions{})
	assert.NilError(t, err)
	cli.PrependReactor("update", "configmaps", func(a k8stesting.Action) (bool, runtime.Object, error) {
		if a.(k8stesting.UpdateAction).GetObject().(*corev1.ConfigMap).ResourceVersion == "1" {
			return true, nil, apierrors.NewConflict(schema.GroupResource{Resource: "configmaps"}, name, nil)
		}
		return false, nil, nil
	})

	assert.NilError(t, s.Put(ctx, instance("web"), time.Hour))
	assert.NilError(t, s.Save(ctx))

	cm, err := cli.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Equal(t, cm.Labels["owner"], "someone", "the other change is kept")
	_, err = reload(t, cli).Get(ctx, "web")
	assert.NilError(t, err)
}

func TestSave_FailureIsRetried(t *testing.T) {
	ctx := t.Context()
	s, cli := newStore(t)
	assert.NilError(t, s.Load(ctx))
	fail := true
	cli.PrependReactor("create", "configmaps", func(k8stesting.Action) (bool, runtime.Object, error) {
		if fail {
			return true, nil, apierrors.NewServiceUnavailable("down")
		}
		return false, nil, nil
	})

	assert.NilError(t, s.Put(ctx, instance("web"), time.Hour))
	assert.ErrorContains(t, s.Save(ctx), "down")
	assert.Assert(t, s.pending.Load(), "a failed write stays pending")

	fail = false
	assert.NilError(t, s.Save(ctx))
	_, err := reload(t, cli).Get(ctx, "web")
	assert.NilError(t, err)
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	for !cond() {
		select {
		case <-ctx.Done():
			t.Fatal("condition not met")
		case <-time.After(10 * time.Millisecond):
		}
	}
}
