package kubernetes

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/sablier"
	"gotest.tools/v3/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func appPod(name string, status corev1.PodStatus) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ns", Labels: map[string]string{"app": "web"}},
		Status:     status,
	}
}

func waiting(name, reason, message string, restarts int32) corev1.ContainerStatus {
	return corev1.ContainerStatus{
		Name:         name,
		RestartCount: restarts,
		State:        corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: reason, Message: message}},
	}
}

func TestPodStatusProblem(t *testing.T) {
	tests := []struct {
		name   string
		status corev1.PodStatus
		want   string
	}{
		{
			name: "unschedulable",
			status: corev1.PodStatus{Conditions: []corev1.PodCondition{{
				Type: corev1.PodScheduled, Status: corev1.ConditionFalse, Reason: corev1.PodReasonUnschedulable,
				Message: "0/3 nodes are available: 3 Insufficient memory.",
			}}},
			want: "Unschedulable: pod web-0: 0/3 nodes are available: 3 Insufficient memory.",
		},
		{
			name: "crash loop reports how the container ended",
			status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{{
				Name:                 "app",
				RestartCount:         4,
				State:                corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff", Message: "back-off 1m20s restarting failed container"}},
				LastTerminationState: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: "OOMKilled", ExitCode: 137}},
			}}},
			want: "CrashLoopBackOff: container app of pod web-0, 4 restarts: last exit OOMKilled (exit code 137)",
		},
		{
			name: "image pull in an init container",
			status: corev1.PodStatus{InitContainerStatuses: []corev1.ContainerStatus{
				waiting("migrate", "ImagePullBackOff", `Back-off pulling image "registry.example/app:v2"`, 0),
			}},
			want: `ImagePullBackOff: container migrate of pod web-0: Back-off pulling image "registry.example/app:v2"`,
		},
		{
			name: "missing secret",
			status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{
				waiting("app", "CreateContainerConfigError", `secret "db" not found`, 0),
			}},
			want: `CreateContainerConfigError: container app of pod web-0: secret "db" not found`,
		},
		{
			name: "still creating is not a problem",
			status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{
				waiting("app", "ContainerCreating", "", 0),
			}},
			want: "",
		},
		{
			name: "scheduled and running",
			status: corev1.PodStatus{
				Conditions:        []corev1.PodCondition{{Type: corev1.PodScheduled, Status: corev1.ConditionTrue}},
				ContainerStatuses: []corev1.ContainerStatus{{Name: "app", State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}},
			},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, podStatusProblem(appPod("web-0", tt.status)), tt.want)
		})
	}
}

func startingDeployment() *appsv1.Deployment {
	replicas := int32(1)
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "ns"},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}},
		},
	}
}

func TestDeploymentInspect_ReportsPodProblem(t *testing.T) {
	ctx := context.Background()
	stuck := appPod("web-7d9-abc", corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{
		waiting("app", "ImagePullBackOff", `Back-off pulling image "app:v2"`, 0),
	}})
	other := appPod("other-0", corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{
		waiting("app", "CrashLoopBackOff", "", 3),
	}})
	other.Labels = map[string]string{"app": "other"}
	client := k8sfake.NewSimpleClientset(startingDeployment(), stuck, other)
	parsed, err := ParseName("deployment_ns_web_1", ParseOptions{Delimiter: "_"})
	assert.NilError(t, err)

	p := &Provider{Client: client, l: slogt.New(t), reportPodProblems: true}
	info, err := p.DeploymentInspect(ctx, parsed)
	assert.NilError(t, err)
	// The instance stays starting: a stuck pod may still recover.
	assert.Equal(t, info.Status, sablier.InstanceStatusStarting)
	assert.Equal(t, info.Message, `ImagePullBackOff: container app of pod web-7d9-abc: Back-off pulling image "app:v2"`)

	// Off by default: no pod is listed.
	p = &Provider{Client: client, l: slogt.New(t)}
	client.ClearActions()
	info, err = p.DeploymentInspect(ctx, parsed)
	assert.NilError(t, err)
	assert.Equal(t, info.Message, "")
	for _, a := range client.Actions() {
		assert.Assert(t, a.GetResource().Resource != "pods", "pods listed while the option is off")
	}
}

func TestStatefulSetInspect_ReportsPodProblem(t *testing.T) {
	ctx := context.Background()
	replicas := int32(1)
	ss := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "ns"},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}},
		},
	}
	pending := appPod("web-0", corev1.PodStatus{Conditions: []corev1.PodCondition{{
		Type: corev1.PodScheduled, Status: corev1.ConditionFalse, Reason: corev1.PodReasonUnschedulable,
		Message: strings.Repeat("0/3 nodes are available. ", 30),
	}}})
	client := k8sfake.NewSimpleClientset(ss, pending)
	parsed, err := ParseName("statefulset_ns_web_1", ParseOptions{Delimiter: "_"})
	assert.NilError(t, err)

	p := &Provider{Client: client, l: slogt.New(t), reportPodProblems: true}
	info, err := p.StatefulSetInspect(ctx, parsed)
	assert.NilError(t, err)
	assert.Equal(t, info.Status, sablier.InstanceStatusStarting)
	assert.Assert(t, strings.HasPrefix(info.Message, "Unschedulable: pod web-0: 0/3 nodes are available."))
	assert.Assert(t, len(info.Message) <= maxPodProblemLength+len("…"))
}

// Listing pods is a courtesy: without the permission, the inspection still
// reports the workload, without an explanation.
func TestDeploymentInspect_PodListForbidden(t *testing.T) {
	client := k8sfake.NewSimpleClientset(startingDeployment())
	client.PrependReactor("list", "pods", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: "pods"}, "", errors.New("RBAC"))
	})
	parsed, err := ParseName("deployment_ns_web_1", ParseOptions{Delimiter: "_"})
	assert.NilError(t, err)

	p := &Provider{Client: client, l: slogt.New(t), reportPodProblems: true}
	info, err := p.DeploymentInspect(context.Background(), parsed)
	assert.NilError(t, err)
	assert.Equal(t, info.Status, sablier.InstanceStatusStarting)
	assert.Equal(t, info.Message, "")
}
