package kubernetes_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/config"
	"github.com/sablierapp/sablier/pkg/provider/kubernetes"
	"github.com/sablierapp/sablier/pkg/sablier"
	"gotest.tools/v3/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const blockedBySablierTest = "blocked by the sablier test"

// blockEndpointSlices makes the API server reject every write of the
// EndpointSlices of service, so the EndpointSlice controller cannot route to
// its pods, until the returned function lifts the block.
func blockEndpointSlices(ctx context.Context, t *testing.T, c *kindContainer, namespace, service string) (unblock func()) {
	t.Helper()
	name := "sablier-test-block-" + service
	fail := admissionregistrationv1.Fail
	_, err := c.client.AdmissionregistrationV1().ValidatingAdmissionPolicies().Create(ctx, &admissionregistrationv1.ValidatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
			FailurePolicy: &fail,
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ObjectSelector: &metav1.LabelSelector{MatchLabels: map[string]string{discoveryv1.LabelServiceName: service}},
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
					RuleWithOperations: admissionregistrationv1.RuleWithOperations{
						Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{discoveryv1.GroupName},
							APIVersions: []string{"v1"},
							Resources:   []string{"endpointslices"},
						},
					},
				}},
			},
			Validations: []admissionregistrationv1.Validation{{Expression: "false", Message: blockedBySablierTest}},
		},
	}, metav1.CreateOptions{})
	assert.NilError(t, err)
	_, err = c.client.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Create(ctx, &admissionregistrationv1.ValidatingAdmissionPolicyBinding{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{
			PolicyName:        name,
			ValidationActions: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny},
		},
	}, metav1.CreateOptions{})
	assert.NilError(t, err)

	unblock = func() {
		_ = c.client.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Delete(context.Background(), name, metav1.DeleteOptions{})
		_ = c.client.AdmissionregistrationV1().ValidatingAdmissionPolicies().Delete(context.Background(), name, metav1.DeleteOptions{})
	}
	t.Cleanup(unblock)

	// The API server loads the policy asynchronously: wait until it rejects
	// a dry-run write.
	probe := &discoveryv1.EndpointSlice{
		ObjectMeta:  metav1.ObjectMeta{Name: name + "-probe", Namespace: namespace, Labels: map[string]string{discoveryv1.LabelServiceName: service}},
		AddressType: discoveryv1.AddressTypeIPv4,
	}
	for {
		_, err := c.client.DiscoveryV1().EndpointSlices(namespace).Create(ctx, probe, metav1.CreateOptions{DryRun: []string{metav1.DryRunAll}})
		if err != nil && strings.Contains(err.Error(), blockedBySablierTest) {
			return unblock
		}
		select {
		case <-ctx.Done():
			t.Fatalf("the admission policy is not enforced: %v", err)
		case <-time.After(200 * time.Millisecond):
		}
	}
}

// With --provider.kubernetes.readiness=endpoints, a Deployment whose pod is
// ready stays starting while its Service does not route to the pod, and turns
// ready once the EndpointSlice controller lists the pod as a ready endpoint.
// An admission policy holds the controller back, so the test does not depend
// on how fast it is.
func TestKubernetesProvider_InstanceInspectReadiness_Endpoints(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	c := sharedKinD

	d, err := c.CreateMimicDeployment(ctx, MimicOptions{Cmd: []string{"/mimic"}})
	assert.NilError(t, err)
	t.Cleanup(func() {
		_ = c.client.AppsV1().Deployments(d.Namespace).Delete(context.Background(), d.Name, metav1.DeleteOptions{})
	})

	unblock := blockEndpointSlices(ctx, t, c, d.Namespace, d.Name)

	svc, err := c.client.CoreV1().Services(d.Namespace).Create(ctx, &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: d.Name, Namespace: d.Namespace},
		Spec: corev1.ServiceSpec{
			Selector: d.Spec.Template.Labels,
			Ports:    []corev1.ServicePort{{Port: 80, TargetPort: intstr.FromInt32(80)}},
		},
	}, metav1.CreateOptions{})
	assert.NilError(t, err)
	t.Cleanup(func() {
		_ = c.client.CoreV1().Services(svc.Namespace).Delete(context.Background(), svc.Name, metav1.DeleteOptions{})
	})

	assert.NilError(t, WaitForDeploymentReady(ctx, c.client, d.Namespace, d.Name))

	cfg := config.NewProviderConfig().Kubernetes
	cfg.Readiness = config.KubernetesReadinessEndpoints
	p, err := kubernetes.New(ctx, c.client, c.dynamic, slogt.New(t), cfg)
	assert.NilError(t, err)
	name := kubernetes.DeploymentName(d, kubernetes.ParseOptions{Delimiter: "_"}).Original

	// The pod is ready, but the Service has no EndpointSlice to route to it.
	got, err := p.InstanceInspectReadiness(ctx, name)
	assert.NilError(t, err)
	assert.Equal(t, got.Status, sablier.InstanceStatusStarting)
	assert.Equal(t, got.Message, fmt.Sprintf("waiting for service %q to route to the workload: 0 of 1 pods routed", svc.Name))
	assert.Equal(t, got.CurrentReplicas, int32(0))

	// The other callers of inspect still report the replicas.
	got, err = p.InstanceInspect(ctx, name)
	assert.NilError(t, err)
	assert.Equal(t, got.Status, sablier.InstanceStatusReady)

	// Once the controller may write the EndpointSlice, the Service routes to
	// the pod. Updating the Service makes the controller sync it again at
	// once, instead of after its retry backoff.
	unblock()
	for i := 0; ; i++ {
		got, err = p.InstanceInspectReadiness(ctx, name)
		assert.NilError(t, err)
		if got.Status == sablier.InstanceStatusReady {
			break
		}
		assert.Equal(t, got.Status, sablier.InstanceStatusStarting)
		_, err = c.client.CoreV1().Services(svc.Namespace).Patch(ctx, svc.Name, types.MergePatchType,
			fmt.Appendf(nil, `{"metadata":{"annotations":{"sablier.test/sync":"%d"}}}`, i), metav1.PatchOptions{})
		assert.NilError(t, err)
		select {
		case <-ctx.Done():
			t.Fatalf("the deployment is still %s: %s", got.Status, got.Message)
		case <-time.After(500 * time.Millisecond):
		}
	}
	assert.Equal(t, got.Message, "")
	assert.Equal(t, got.CurrentReplicas, int32(1))
}
