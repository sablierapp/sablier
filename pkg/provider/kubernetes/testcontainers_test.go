package kubernetes_test

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/k3s"
	"gotest.tools/v3/assert"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"math/rand"
	"testing"
	"time"
)

var r = rand.New(rand.NewSource(time.Now().UnixNano()))

type kindContainer struct {
	testcontainers.Container
	client *kubernetes.Clientset
	t      *testing.T
}

type MimicOptions struct {
	Cmd         []string
	Healthcheck *corev1.Probe
	Labels      map[string]string
}

func (d *kindContainer) CreateMimicDeployment(ctx context.Context, opts MimicOptions) (*v1.Deployment, error) {
	if len(opts.Cmd) == 0 {
		opts.Cmd = []string{"/mimic", "-running", "-running-after=1s", "-healthy=false"}
	}

	name := generateRandomName()
	// Add the app label to the deployment for matching the selector
	if opts.Labels == nil {
		opts.Labels = make(map[string]string)
	}
	opts.Labels["app"] = name
	d.t.Log("Creating mimic deployment with options", opts)
	replicas := int32(1)
	return d.client.AppsV1().Deployments("default").Create(ctx, &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": name,
				},
			},
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:    "mimic",
							Image:   "sablierapp/mimic:v0.3.1",
							Command: opts.Cmd,
							// ReadinessProbe: opts.Healthcheck,
						},
					},
				},
				ObjectMeta: metav1.ObjectMeta{
					Labels: opts.Labels,
				},
			},
		},
	}, metav1.CreateOptions{})
}

func (d *kindContainer) CreateMimicStatefulSet(ctx context.Context, opts MimicOptions) (*v1.StatefulSet, error) {
	if len(opts.Cmd) == 0 {
		opts.Cmd = []string{"/mimic", "-running", "-running-after=1s", "-healthy=false"}
	}

	name := generateRandomName()
	// Add the app label to the deployment for matching the selector
	if opts.Labels == nil {
		opts.Labels = make(map[string]string)
	}
	opts.Labels["app"] = name
	d.t.Log("Creating mimic deployment with options", opts)
	replicas := int32(1)
	return d.client.AppsV1().StatefulSets("default").Create(ctx, &v1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1.StatefulSetSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": name,
				},
			},
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:    "mimic",
							Image:   "sablierapp/mimic:v0.3.1",
							Command: opts.Cmd,
							// ReadinessProbe: opts.Healthcheck,
						},
					},
				},
				ObjectMeta: metav1.ObjectMeta{
					Labels: opts.Labels,
				},
			},
		},
	}, metav1.CreateOptions{})
}

func setupKinD(t *testing.T, ctx context.Context) *kindContainer {
	t.Helper()

	kind, err := k3s.Run(ctx, "rancher/k3s:v1.27.1-k3s1")
	testcontainers.CleanupContainer(t, kind)
	assert.NilError(t, err)

	kubeConfigYaml, err := kind.GetKubeConfig(ctx)
	assert.NilError(t, err)

	restcfg, err := clientcmd.RESTConfigFromKubeConfig(kubeConfigYaml)
	assert.NilError(t, err)

	provider, err := testcontainers.ProviderDocker.GetProvider()
	assert.NilError(t, err)

	err = provider.PullImage(ctx, "sablierapp/mimic:v0.3.1")
	require.NoError(t, err)

	err = kind.LoadImages(ctx, "sablierapp/mimic:v0.3.1")
	assert.NilError(t, err)

	k8s, err := kubernetes.NewForConfig(restcfg)
	assert.NilError(t, err)

	return &kindContainer{
		Container: kind,
		client:    k8s,
		t:         t,
	}
}

func generateRandomName() string {
	letters := []rune("abcdefghijklmnopqrstuvwxyz")
	name := make([]rune, 10)
	for i := range name {
		name[i] = letters[r.Intn(len(letters))]
	}
	return string(name)
}

func WaitForDeploymentReady(ctx context.Context, client kubernetes.Interface, namespace, name string) error {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context canceled while waiting for deployment %s/%s", namespace, name)
		case <-ticker.C:
			deployment, err := client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("error getting deployment: %w", err)
			}

			if deployment.Status.ReadyReplicas == *deployment.Spec.Replicas {
				return nil
			}
		}
	}
}

func WaitForStatefulSetReady(ctx context.Context, client kubernetes.Interface, namespace, name string) error {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context canceled while waiting for statefulSet %s/%s", namespace, name)
		case <-ticker.C:
			statefulSet, err := client.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("error getting statefulSet: %w", err)
			}

			if statefulSet.Status.ReadyReplicas == *statefulSet.Spec.Replicas {
				return nil
			}
		}
	}
}

func WaitForDeploymentScale(ctx context.Context, client kubernetes.Interface, namespace, name string, replicas int32) error {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context canceled while waiting for deployment %s/%s scale", namespace, name)
		case <-ticker.C:
			deployment, err := client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("error getting deployment: %w", err)
			}

			if *deployment.Spec.Replicas == replicas && deployment.Status.Replicas == replicas {
				return nil
			}
		}
	}
}

func WaitForStatefulSetScale(ctx context.Context, client kubernetes.Interface, namespace, name string, replicas int32) error {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context canceled while waiting for statefulSet %s/%s scale", namespace, name)
		case <-ticker.C:
			statefulSet, err := client.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("error getting statefulSet: %w", err)
			}

			if *statefulSet.Spec.Replicas == replicas && statefulSet.Status.Replicas == replicas {
				return nil
			}
		}
	}
}
