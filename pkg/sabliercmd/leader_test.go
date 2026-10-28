package sabliercmd

import (
	"context"
	"testing"

	"github.com/neilotoole/slogt"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestSetLeaderLabel(t *testing.T) {
	ctx := context.Background()
	cli := fake.NewClientset(&corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name: "sablier-0", Namespace: "sablier",
		Labels: map[string]string{"app": "sablier", LeaderLabel: "true"},
	}})
	le := &leaderElection{cli: cli, l: slogt.New(t), pod: "sablier-0", namespace: "sablier"}
	labels := func() map[string]string {
		pod, err := cli.CoreV1().Pods("sablier").Get(ctx, "sablier-0", metav1.GetOptions{})
		assert.NilError(t, err)
		return pod.Labels
	}

	// A restarted leader starts as a follower: the stale label must go.
	assert.NilError(t, le.setLeaderLabel(ctx, false))
	assert.DeepEqual(t, labels(), map[string]string{"app": "sablier"})

	assert.NilError(t, le.setLeaderLabel(ctx, true))
	assert.DeepEqual(t, labels(), map[string]string{"app": "sablier", LeaderLabel: "true"})
}

func TestSetLeaderLabel_MissingPod(t *testing.T) {
	le := &leaderElection{cli: fake.NewClientset(), l: slogt.New(t), pod: "gone", namespace: "sablier"}
	assert.ErrorContains(t, le.setLeaderLabel(context.Background(), true), "cannot label pod sablier/gone")
}

func TestPodNamespace(t *testing.T) {
	t.Setenv("POD_NAMESPACE", "from-env")
	ns, err := podNamespace()
	assert.NilError(t, err)
	assert.Equal(t, ns, "from-env")
}
