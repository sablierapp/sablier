package kubernetes_test

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/k3s"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var r = rand.New(rand.NewSource(time.Now().UnixNano()))
var mu sync.Mutex // r is not safe for concurrent use

// sharedKinD is the single k3s cluster shared across all tests in this package.
// It is initialized by TestMain, which avoids the overhead of starting a new cluster per test.
var sharedKinD *kindContainer

type kindContainer struct {
	testcontainers.Container
	client  *kubernetes.Clientset
	dynamic dynamic.Interface
	restcfg *rest.Config
}

type MimicOptions struct {
	Cmd         []string
	Healthcheck *corev1.Probe
	Labels      map[string]string
	Annotations map[string]string
}

// mimicHealthcheck returns a readiness probe wired to the mimic health
// endpoint, so pod readiness follows the -healthy/-healthy-after flags
// instead of flipping to Ready as soon as the container is running.
func mimicHealthcheck() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			Exec: &corev1.ExecAction{
				Command: []string{"/mimic", "healthcheck"},
			},
		},
		PeriodSeconds: 1,
	}
}

func (d *kindContainer) CreateMimicDeployment(ctx context.Context, opts MimicOptions) (*v1.Deployment, error) {
	if len(opts.Cmd) == 0 {
		opts.Cmd = []string{"/mimic", "-running", "-running-after=1s", "-healthy=false"}
	}

	name := generateRandomName()
	if opts.Labels == nil {
		opts.Labels = make(map[string]string)
	}
	replicas := int32(1)
	return d.client.AppsV1().Deployments("default").Create(ctx, &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Labels:      opts.Labels,
			Annotations: opts.Annotations,
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
							Name:           "mimic",
							Image:          "sablierapp/mimic:v0.3.3",
							Command:        opts.Cmd,
							ReadinessProbe: opts.Healthcheck,
						},
					},
				},
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": name,
					},
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
	if opts.Labels == nil {
		opts.Labels = make(map[string]string)
	}
	replicas := int32(1)
	return d.client.AppsV1().StatefulSets("default").Create(ctx, &v1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Labels:      opts.Labels,
			Annotations: opts.Annotations,
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
							Name:           "mimic",
							Image:          "sablierapp/mimic:v0.3.3",
							Command:        opts.Cmd,
							ReadinessProbe: opts.Healthcheck,
						},
					},
				},
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": name,
					},
				},
			},
		},
	}, metav1.CreateOptions{})
}

func TestMain(m *testing.M) {
	// flag.Parse must be called before testing.Short() is usable.
	flag.Parse()

	// Skip the expensive container setup when running in short mode.
	if testing.Short() {
		os.Exit(m.Run())
	}

	ctx := context.Background()

	kind, err := k3s.Run(ctx, "rancher/k3s:v1.36.0-k3s1")
	if err != nil {
		log.Fatalf("failed to start k3s: %v", err)
	}

	kubeConfigYaml, err := kind.GetKubeConfig(ctx)
	if err != nil {
		_ = kind.Terminate(ctx)
		log.Fatalf("failed to get kubeconfig: %v", err)
	}

	restcfg, err := clientcmd.RESTConfigFromKubeConfig(kubeConfigYaml)
	if err != nil {
		_ = kind.Terminate(ctx)
		log.Fatalf("failed to build rest config: %v", err)
	}
	// Raise rate limits to avoid client-side throttling when tests run in parallel.
	restcfg.QPS = 100
	restcfg.Burst = 200

	provider, err := testcontainers.ProviderDocker.GetProvider()
	if err != nil {
		_ = kind.Terminate(ctx)
		log.Fatalf("failed to get docker provider: %v", err)
	}

	if err = provider.PullImage(ctx, "sablierapp/mimic:v0.3.3"); err != nil {
		_ = kind.Terminate(ctx)
		log.Fatalf("failed to pull mimic image: %v", err)
	}

	if err = kind.LoadImages(ctx, "sablierapp/mimic:v0.3.3"); err != nil {
		_ = kind.Terminate(ctx)
		log.Fatalf("failed to load mimic image: %v", err)
	}

	k8s, err := kubernetes.NewForConfig(restcfg)
	if err != nil {
		_ = kind.Terminate(ctx)
		log.Fatalf("failed to create kubernetes client: %v", err)
	}

	dyn, err := dynamic.NewForConfig(restcfg)
	if err != nil {
		_ = kind.Terminate(ctx)
		log.Fatalf("failed to create dynamic client: %v", err)
	}

	sharedKinD = &kindContainer{
		Container: kind,
		client:    k8s,
		dynamic:   dyn,
		restcfg:   restcfg,
	}

	code := m.Run()
	_ = kind.Terminate(ctx)
	os.Exit(code)
}

func generateRandomName() string {
	mu.Lock()
	defer mu.Unlock()
	letters := []rune("abcdefghijklmnopqrstuvwxyz")
	name := make([]rune, 10)
	for i := range name {
		name[i] = letters[r.Intn(len(letters))]
	}
	return string(name)
}

func WaitForDeploymentReady(ctx context.Context, client kubernetes.Interface, namespace, name string) error {
	ticker := time.NewTicker(100 * time.Millisecond)
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
	ticker := time.NewTicker(100 * time.Millisecond)
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
	ticker := time.NewTicker(100 * time.Millisecond)
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
	ticker := time.NewTicker(100 * time.Millisecond)
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
