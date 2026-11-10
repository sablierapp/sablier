package kubernetes

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"slices"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"

	"github.com/sablierapp/sablier/pkg/sablier"
)

// Interface guard
var _ sablier.ReadinessInspector = (*Provider)(nil)

// InstanceInspectReadiness implements sablier.ReadinessInspector. With
// --provider.kubernetes.readiness=endpoints, a Deployment or StatefulSet whose
// replicas are ready stays starting until its Services route to its pods.
// Otherwise, and for CloudNativePG Clusters, it is InstanceInspect.
func (p *Provider) InstanceInspectReadiness(ctx context.Context, name string) (sablier.InstanceInfo, error) {
	if !p.readyOnEndpoints {
		return p.InstanceInspect(ctx, name)
	}
	parsed, err := ParseName(name, ParseOptions{Delimiter: p.delimiter})
	if err != nil {
		return sablier.InstanceInfo{}, err
	}

	switch parsed.Kind {
	case "deployment":
		d, info, err := p.deploymentInspect(ctx, parsed)
		if err != nil {
			return sablier.InstanceInfo{}, err
		}
		return p.routedInfo(ctx, info, workload{
			namespace: d.Namespace,
			selector:  d.Spec.Selector,
			replicas:  replicasOf(d.Spec.Replicas),
			owns:      ownedByDeployment(d),
		}), nil
	case "statefulset":
		ss, info, err := p.statefulSetInspect(ctx, parsed)
		if err != nil {
			return sablier.InstanceInfo{}, err
		}
		return p.routedInfo(ctx, info, workload{
			namespace: ss.Namespace,
			selector:  ss.Spec.Selector,
			replicas:  replicasOf(ss.Spec.Replicas),
			owns:      ownedByStatefulSet(ss),
		}), nil
	default:
		return p.InstanceInspect(ctx, name)
	}
}

// workload is what routing needs to know about a Deployment or StatefulSet.
type workload struct {
	namespace string
	selector  *metav1.LabelSelector
	replicas  int32
	// owns reports whether the workload's controller manages pod. Its label
	// selector also matches the pods of a workload with overlapping labels,
	// such as a canary.
	owns func(pod *corev1.Pod) bool
}

// ownedByDeployment reports whether a pod belongs to d. A Deployment manages
// its pods through ReplicaSets named <deployment>-<pod-template-hash>, the
// hash being a label of their pods, so the ReplicaSets need not be read.
func ownedByDeployment(d *appsv1.Deployment) func(*corev1.Pod) bool {
	return func(pod *corev1.Pod) bool {
		ref := metav1.GetControllerOf(pod)
		hash := pod.Labels[appsv1.DefaultDeploymentUniqueLabelKey]
		return ref != nil && ref.Kind == "ReplicaSet" && hash != "" && ref.Name == d.Name+"-"+hash
	}
}

func ownedByStatefulSet(ss *appsv1.StatefulSet) func(*corev1.Pod) bool {
	return func(pod *corev1.Pod) bool {
		ref := metav1.GetControllerOf(pod)
		return ref != nil && ref.Kind == "StatefulSet" && ref.UID == ss.UID
	}
}

// routedInfo keeps info, ready from its replicas, in the starting status until
// enough of the workload's pods are routed: all desired replicas, or one with
// ready-on-first-replica. A pod turns Ready before the EndpointSlice controller
// marks its endpoint ready, and before kube-proxy and the reverse proxies apply
// that change: a request released in between fails with a 502 or a 503.
func (p *Provider) routedInfo(ctx context.Context, info sablier.InstanceInfo, w workload) sablier.InstanceInfo {
	// A workload that is still starting costs no extra request.
	if info.Status != sablier.InstanceStatusReady {
		return info
	}
	required := w.replicas
	if p.readyOnFirstReplica {
		required = 1
	}

	r, err := p.routing(ctx, w)
	switch {
	case err == nil:
	case apierrors.IsForbidden(err) || apierrors.IsNotFound(err):
		// Sablier cannot read the Services or their endpoints, and will not
		// be able to on the next request either: report the workload from its
		// replicas, as with --provider.kubernetes.readiness=replicas.
		if p.routingFallbackWarned.CompareAndSwap(false, true) {
			p.l.WarnContext(ctx, "cannot check whether the services route to the workloads, reporting them ready from their replicas: grant list on services, pods and endpointslices.discovery.k8s.io",
				slog.String("instance", info.Name), slog.Any("error", err))
		} else {
			p.l.DebugContext(ctx, "cannot check whether the services route to the workload, reporting it ready from its replicas",
				slog.String("instance", info.Name), slog.Any("error", err))
		}
		return info
	default:
		// A cancelled request or a throttled API call must not release the
		// waiting requests: a ready status is stored in the session and never
		// checked again. The next request checks again.
		info.Status = sablier.InstanceStatusStarting
		info.Message = fmt.Sprintf("cannot check whether the services route to the workload: %v", err)
		return info
	}

	if r.routed >= required {
		return info
	}
	info.Status = sablier.InstanceStatusStarting
	info.CurrentReplicas = r.routed
	if r.service != "" {
		info.Message = fmt.Sprintf("waiting for service %q to route to the workload: %d of %d pods routed", r.service, r.routed, required)
	} else {
		// The workload's status counts ready replicas that its pods do not
		// show yet.
		info.Message = fmt.Sprintf("waiting for the pods of the workload: %d of %d ready", r.routed, required)
	}
	return info
}

// routingState is what routing found.
type routingState struct {
	// routed counts the ready pods of the workload that every Service
	// selecting them lists as ready endpoints.
	routed int32
	// service is a Service that does not route to one of the ready pods yet.
	service string
}

// routing counts the ready pods of w that every Service selecting them routes
// to, as their EndpointSlices show. A pod that no Service selects counts once
// ready. It costs three list requests: the pods, the Services and the
// EndpointSlices of the Services that select them.
func (p *Provider) routing(ctx context.Context, w workload) (routingState, error) {
	selector, err := metav1.LabelSelectorAsSelector(w.selector)
	if err != nil {
		return routingState{}, fmt.Errorf("parse workload selector: %w", err)
	}
	podList, err := p.Client.CoreV1().Pods(w.namespace).List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return routingState{}, fmt.Errorf("list pods: %w", err)
	}
	var pods []*corev1.Pod
	for i := range podList.Items {
		if pod := &podList.Items[i]; w.owns(pod) && podReady(pod) {
			pods = append(pods, pod)
		}
	}
	if len(pods) == 0 {
		return routingState{}, nil
	}
	slices.SortFunc(pods, func(a, b *corev1.Pod) int { return cmp.Compare(a.Name, b.Name) })

	serviceList, err := p.Client.CoreV1().Services(w.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return routingState{}, fmt.Errorf("list services: %w", err)
	}
	services := serviceList.Items
	slices.SortFunc(services, func(a, b corev1.Service) int { return cmp.Compare(a.Name, b.Name) })

	// The Services that select each pod. A Service is matched against the
	// labels of the pods, not of the pod template: a controller adds labels,
	// such as statefulset.kubernetes.io/pod-name, that a per-replica Service
	// selects.
	selecting := make(map[string][]string, len(pods))
	var names []string
	for i := range services {
		svc := &services[i]
		// The EndpointSlice controller never writes endpoints for a Service
		// without a selector, or for an ExternalName Service even with one:
		// waiting for them would never end.
		if len(svc.Spec.Selector) == 0 || svc.Spec.Type == corev1.ServiceTypeExternalName {
			continue
		}
		svcSelector := labels.SelectorFromSet(svc.Spec.Selector)
		matched := false
		for _, pod := range pods {
			if svcSelector.Matches(labels.Set(pod.Labels)) {
				selecting[pod.Name] = append(selecting[pod.Name], svc.Name)
				matched = true
			}
		}
		if matched {
			names = append(names, svc.Name)
		}
	}

	routed, err := p.readyEndpoints(ctx, w.namespace, names, pods)
	if err != nil {
		return routingState{}, err
	}

	var state routingState
	for _, pod := range pods {
		blocking := ""
		for _, svc := range selecting[pod.Name] {
			if _, ok := routed[svc][pod.Name]; !ok {
				blocking = svc
				break
			}
		}
		if blocking == "" {
			state.routed++
		} else if state.service == "" {
			state.service = blocking
		}
	}
	return state, nil
}

// readyEndpoints returns, for each of the services, the pods that it lists as
// ready endpoints, in one request.
func (p *Provider) readyEndpoints(ctx context.Context, namespace string, services []string, pods []*corev1.Pod) (map[string]map[string]struct{}, error) {
	routed := make(map[string]map[string]struct{}, len(services))
	if len(services) == 0 {
		return routed, nil
	}
	ofServices, err := labels.NewRequirement(discoveryv1.LabelServiceName, selection.In, services)
	if err != nil {
		return nil, fmt.Errorf("select the endpointslices of services %v: %w", services, err)
	}
	epSlices, err := p.Client.DiscoveryV1().EndpointSlices(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.NewSelector().Add(*ofServices).String(),
	})
	if err != nil {
		return nil, fmt.Errorf("list endpointslices: %w", err)
	}

	uids := make(map[string]types.UID, len(pods))
	for _, pod := range pods {
		uids[pod.Name] = pod.UID
	}
	// A dual-stack Service lists each pod in one slice per address family:
	// the sets count each pod once.
	for i := range epSlices.Items {
		epSlice := &epSlices.Items[i]
		svc := epSlice.Labels[discoveryv1.LabelServiceName]
		for j := range epSlice.Endpoints {
			ep := &epSlice.Endpoints[j]
			if !endpointReady(ep) || !targetsPod(uids, ep.TargetRef) {
				continue
			}
			if routed[svc] == nil {
				routed[svc] = make(map[string]struct{})
			}
			routed[svc][ep.TargetRef.Name] = struct{}{}
		}
	}
	return routed, nil
}

// podReady reports whether pod passes its readiness checks and is not
// terminating, like the ready replicas of its workload.
func podReady(pod *corev1.Pod) bool {
	if pod.DeletionTimestamp != nil {
		return false
	}
	for _, c := range pod.Status.Conditions {
		if c.Type == corev1.PodReady {
			return c.Status == corev1.ConditionTrue
		}
	}
	return false
}

// endpointReady reports whether ep receives traffic. An unset ready condition
// means ready, as the EndpointSlice API documents.
func endpointReady(ep *discoveryv1.Endpoint) bool {
	return ep.Conditions.Ready == nil || *ep.Conditions.Ready
}

// targetsPod reports whether ref targets one of the pods, given by name and
// UID. A pod that was recreated with the same name, as a StatefulSet does, has
// a new UID: an endpoint left over from the former pod does not count.
func targetsPod(uids map[string]types.UID, ref *corev1.ObjectReference) bool {
	if ref == nil || ref.Kind != "Pod" {
		return false
	}
	uid, ok := uids[ref.Name]
	return ok && (ref.UID == "" || ref.UID == uid)
}
