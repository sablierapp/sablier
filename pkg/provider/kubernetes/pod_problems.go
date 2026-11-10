package kubernetes

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"unicode/utf8"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// blockingWaitingReasons are the reasons for which a container keeps waiting
// instead of running, and that waiting a little longer will not resolve,
// unlike ContainerCreating or PodInitializing.
var blockingWaitingReasons = map[string]bool{
	"CrashLoopBackOff":           true,
	"ImagePullBackOff":           true,
	"ErrImagePull":               true,
	"InvalidImageName":           true,
	"CreateContainerConfigError": true,
	"CreateContainerError":       true,
	"RunContainerError":          true,
}

// maxPodProblemLength bounds the message reported for a pod problem: a
// scheduler message lists every node of the cluster.
const maxPodProblemLength = 300

// podProblem tells why the pods selected by selector in namespace do not
// become ready, when one of them is stuck in a state that waiting will not
// resolve: it cannot be scheduled, an image cannot be pulled, a container
// keeps crashing... It returns "" when no pod is stuck, and when the pods
// cannot be listed: the explanation is a courtesy, never a reason to fail the
// inspection.
func (p *Provider) podProblem(ctx context.Context, namespace string, selector *metav1.LabelSelector) string {
	sel, err := metav1.LabelSelectorAsSelector(selector)
	if err != nil || sel.Empty() {
		return ""
	}
	pods, err := p.Client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: sel.String()})
	if err != nil {
		p.l.DebugContext(ctx, "cannot list pods to explain why the workload is not ready",
			slog.String("namespace", namespace), slog.Any("error", err))
		return ""
	}

	items := pods.Items
	slices.SortFunc(items, func(a, b corev1.Pod) int { return strings.Compare(a.Name, b.Name) })
	for i := range items {
		if items[i].DeletionTimestamp != nil {
			continue
		}
		if problem := podStatusProblem(&items[i]); problem != "" {
			return truncate(problem, maxPodProblemLength)
		}
	}
	return ""
}

// podStatusProblem describes what keeps pod from running, from its status
// alone, or returns "" when nothing does. The message starts with the
// Kubernetes reason (Unschedulable, CrashLoopBackOff...).
func podStatusProblem(pod *corev1.Pod) string {
	for _, c := range pod.Status.Conditions {
		if c.Type == corev1.PodScheduled && c.Status == corev1.ConditionFalse && c.Reason == corev1.PodReasonUnschedulable {
			return fmt.Sprintf("Unschedulable: pod %s: %s", pod.Name, c.Message)
		}
	}

	statuses := slices.Concat(pod.Status.InitContainerStatuses, pod.Status.ContainerStatuses)
	for _, cs := range statuses {
		w := cs.State.Waiting
		if w == nil || !blockingWaitingReasons[w.Reason] {
			continue
		}
		msg := fmt.Sprintf("%s: container %s of pod %s", w.Reason, cs.Name, pod.Name)
		if cs.RestartCount > 0 {
			msg += fmt.Sprintf(", %d restarts", cs.RestartCount)
		}
		// For a crash loop, how the container ended says more than the
		// kubelet's back-off message.
		if t := cs.LastTerminationState.Terminated; w.Reason == "CrashLoopBackOff" && t != nil {
			return msg + fmt.Sprintf(": last exit %s (exit code %d)", t.Reason, t.ExitCode)
		}
		if w.Message != "" {
			msg += ": " + w.Message
		}
		return msg
	}
	return ""
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	s = s[:n]
	for !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s + "…"
}
