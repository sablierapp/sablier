package kubernetes_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/config"
	"github.com/sablierapp/sablier/pkg/provider/kubernetes"
	"github.com/sablierapp/sablier/pkg/sablier"
	"gotest.tools/v3/assert"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestKubernetesProvider_DeploymentInspect(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	t.Parallel()

	ctx := context.Background()
	type args struct {
		do func(dind *kindContainer) (string, error)
	}
	tests := []struct {
		name       string
		args       args
		want       sablier.InstanceInfo
		wantLabels map[string]string
		wantErr    error
	}{
		{
			name: "deployment with 1/1 replicas",
			args: args{
				do: func(dind *kindContainer) (string, error) {
					d, err := dind.CreateMimicDeployment(ctx, MimicOptions{
						Cmd:         []string{"/mimic"},
						Healthcheck: nil,
					})
					if err != nil {
						return "", err
					}

					if err = WaitForDeploymentReady(ctx, dind.client, "default", d.Name); err != nil {
						return "", fmt.Errorf("error waiting for deployment: %w", err)
					}

					return kubernetes.DeploymentName(d, kubernetes.ParseOptions{Delimiter: "_"}).Original, nil
				},
			},
			want: sablier.InstanceInfo{
				CurrentReplicas: 1,
				DesiredReplicas: 1,
				Status:          sablier.InstanceStatusReady,
			},
			wantErr: nil,
		},
		{
			name: "deployment with 0/1 replicas",
			args: args{
				do: func(dind *kindContainer) (string, error) {
					d, err := dind.CreateMimicDeployment(ctx, MimicOptions{
						Cmd:         []string{"/mimic", "-running-after=1ms", "-healthy=false", "-healthy-after=10s"},
						Healthcheck: mimicHealthcheck(),
					})
					if err != nil {
						return "", err
					}

					return kubernetes.DeploymentName(d, kubernetes.ParseOptions{Delimiter: "_"}).Original, nil
				},
			},
			want: sablier.InstanceInfo{
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          sablier.InstanceStatusStarting,
			},
			wantErr: nil,
		},
		{
			name: "deployment with 0/0 replicas",
			args: args{
				do: func(dind *kindContainer) (string, error) {
					d, err := dind.CreateMimicDeployment(ctx, MimicOptions{})
					if err != nil {
						return "", err
					}

					_, err = dind.client.AppsV1().Deployments(d.Namespace).UpdateScale(ctx, d.Name, &autoscalingv1.Scale{
						ObjectMeta: metav1.ObjectMeta{
							Name: d.Name,
						},
						Spec: autoscalingv1.ScaleSpec{
							Replicas: 0,
						},
					}, metav1.UpdateOptions{})
					if err != nil {
						return "", err
					}

					if err = WaitForDeploymentScale(ctx, dind.client, "default", d.Name, 0); err != nil {
						return "", fmt.Errorf("error waiting for deployment: %w", err)
					}

					return kubernetes.DeploymentName(d, kubernetes.ParseOptions{Delimiter: "_"}).Original, nil
				},
			},
			want: sablier.InstanceInfo{
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          sablier.InstanceStatusStopped,
			},
			wantErr: nil,
		},
		{
			name: "deployment with sablier labels",
			args: args{
				do: func(dind *kindContainer) (string, error) {
					d, err := dind.CreateMimicDeployment(ctx, MimicOptions{
						Cmd: []string{"/mimic"},
						Labels: map[string]string{
							"sablier.enable":         "true",
							"sablier.group":          "myapp",
							"sablier.ready-on-start": "true",
						},
					})
					if err != nil {
						return "", err
					}

					if err = WaitForDeploymentReady(ctx, dind.client, "default", d.Name); err != nil {
						return "", fmt.Errorf("error waiting for deployment: %w", err)
					}

					return kubernetes.DeploymentName(d, kubernetes.ParseOptions{Delimiter: "_"}).Original, nil
				},
			},
			want: sablier.InstanceInfo{
				CurrentReplicas: 1,
				DesiredReplicas: 1,
				Status:          sablier.InstanceStatusReady,
				Enabled:         "true",
				Groups:          []string{"myapp"},
				ReadyOnStart:    true,
			},
			wantLabels: map[string]string{
				"sablier.enable":         "true",
				"sablier.group":          "myapp",
				"sablier.ready-on-start": "true",
			},
			wantErr: nil,
		},
		{
			name: "deployment with sablier config via annotations",
			args: args{
				do: func(dind *kindContainer) (string, error) {
					// sablier.group with a comma-separated value is invalid as a
					// Kubernetes label, so it can only be expressed as an annotation.
					d, err := dind.CreateMimicDeployment(ctx, MimicOptions{
						Cmd: []string{"/mimic"},
						Labels: map[string]string{
							"sablier.enable": "true",
						},
						Annotations: map[string]string{
							"sablier.group":          "group-a,group-b",
							"sablier.ready-on-start": "true",
						},
					})
					if err != nil {
						return "", err
					}

					if err = WaitForDeploymentReady(ctx, dind.client, "default", d.Name); err != nil {
						return "", fmt.Errorf("error waiting for deployment: %w", err)
					}

					return kubernetes.DeploymentName(d, kubernetes.ParseOptions{Delimiter: "_"}).Original, nil
				},
			},
			want: sablier.InstanceInfo{
				CurrentReplicas: 1,
				DesiredReplicas: 1,
				Status:          sablier.InstanceStatusReady,
				Enabled:         "true",
				Groups:          []string{"group-a", "group-b"},
				ReadyOnStart:    true,
			},
			// Kubernetes.Labels reflects the raw workload labels only, not the
			// merged annotation config.
			wantLabels: map[string]string{
				"sablier.enable": "true",
			},
			wantErr: nil,
		},
		{
			name: "deployment annotations override labels",
			args: args{
				do: func(dind *kindContainer) (string, error) {
					d, err := dind.CreateMimicDeployment(ctx, MimicOptions{
						Cmd: []string{"/mimic"},
						Labels: map[string]string{
							"sablier.enable": "true",
							"sablier.group":  "from-label",
						},
						Annotations: map[string]string{
							"sablier.group": "from-annotation",
						},
					})
					if err != nil {
						return "", err
					}

					if err = WaitForDeploymentReady(ctx, dind.client, "default", d.Name); err != nil {
						return "", fmt.Errorf("error waiting for deployment: %w", err)
					}

					return kubernetes.DeploymentName(d, kubernetes.ParseOptions{Delimiter: "_"}).Original, nil
				},
			},
			want: sablier.InstanceInfo{
				CurrentReplicas: 1,
				DesiredReplicas: 1,
				Status:          sablier.InstanceStatusReady,
				Enabled:         "true",
				Groups:          []string{"from-annotation"},
			},
			wantLabels: map[string]string{
				"sablier.enable": "true",
				"sablier.group":  "from-label",
			},
			wantErr: nil,
		},
	}
	c := sharedKinD
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p, err := kubernetes.New(ctx, c.client, c.dynamic, slogt.New(t), config.NewProviderConfig().Kubernetes)
			assert.NilError(t, err)

			name, err := tt.args.do(c)
			assert.NilError(t, err)

			// Clean up the deployment created by this subtest.
			if parsed, parseErr := kubernetes.ParseName(name, kubernetes.ParseOptions{Delimiter: "_"}); parseErr == nil {
				t.Cleanup(func() {
					_ = c.client.AppsV1().Deployments(parsed.Namespace).Delete(context.Background(), parsed.Name, metav1.DeleteOptions{})
				})
			}

			tt.want.Name = name
			tt.want.Provider = "kubernetes"
			labels := tt.wantLabels
			if labels == nil {
				labels = map[string]string{}
			}
			tt.want.Kubernetes = &sablier.KubernetesWorkloadInfo{
				Namespace: "default",
				Kind:      "deployment",
				Image:     "sablierapp/mimic:v0.3.3",
				Labels:    labels,
			}
			// The provider mirrors the parsed label config into Config with the
			// same values as the flat fields each case already declares, so
			// derive the expectation instead of repeating it per case.
			tt.want.Config = &sablier.InstanceConfig{
				Enabled:      tt.want.Enabled == "true",
				Groups:       tt.want.Groups,
				ReadyAfter:   tt.want.ReadyAfter,
				ReadyOnStart: tt.want.ReadyOnStart,
				RunningHours: tt.want.RunningHours,
				RunningDays:  tt.want.RunningDays,
				AntiAffinity: tt.want.AntiAffinity,
				Scale:        tt.want.ScaleConfig,
			}
			got, err := p.InstanceInspect(ctx, name)
			if !cmp.Equal(err, tt.wantErr) {
				t.Errorf("Provider.InstanceInspect() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.DeepEqual(t, got, tt.want)
		})
	}
}

func TestKubernetesProvider_DeploymentInspect_ReadyOnFirstReplica(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	t.Parallel()

	// Bound the waits below so a readiness regression fails this test instead
	// of hanging until the package test timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	c := sharedKinD

	// A pod only becomes healthy 20 seconds after its own start (mimic serves
	// 200 on /health once -healthy-after elapses): the first replica gets
	// ready, and the replica added by the scale-up below stays unready long
	// enough to observe a stable 1/2-ready deployment.
	d, err := c.CreateMimicDeployment(ctx, MimicOptions{
		Cmd:         []string{"/mimic", "-running-after=1ms", "-healthy", "-healthy-after=20s"},
		Healthcheck: mimicHealthcheck(),
	})
	assert.NilError(t, err)
	t.Cleanup(func() {
		_ = c.client.AppsV1().Deployments(d.Namespace).Delete(context.Background(), d.Name, metav1.DeleteOptions{})
	})

	assert.NilError(t, WaitForDeploymentReady(ctx, c.client, "default", d.Name))

	_, err = c.client.AppsV1().Deployments(d.Namespace).UpdateScale(ctx, d.Name, &autoscalingv1.Scale{
		ObjectMeta: metav1.ObjectMeta{Name: d.Name},
		Spec:       autoscalingv1.ScaleSpec{Replicas: 2},
	}, metav1.UpdateOptions{})
	assert.NilError(t, err)
	assert.NilError(t, WaitForDeploymentScale(ctx, c.client, "default", d.Name, 2))

	name := kubernetes.DeploymentName(d, kubernetes.ParseOptions{Delimiter: "_"}).Original

	// Default behavior: 1/2 ready replicas is still starting.
	p, err := kubernetes.New(ctx, c.client, c.dynamic, slogt.New(t), config.NewProviderConfig().Kubernetes)
	assert.NilError(t, err)
	got, err := p.InstanceInspect(ctx, name)
	assert.NilError(t, err)
	assert.Equal(t, got.Status, sablier.InstanceStatusStarting)

	// With ready-on-first-replica, one ready replica is enough.
	cfg := config.NewProviderConfig().Kubernetes
	cfg.ReadyOnFirstReplica = true
	p, err = kubernetes.New(ctx, c.client, c.dynamic, slogt.New(t), cfg)
	assert.NilError(t, err)
	got, err = p.InstanceInspect(ctx, name)
	assert.NilError(t, err)
	assert.Equal(t, got.Status, sablier.InstanceStatusReady)
	assert.Equal(t, got.CurrentReplicas, int32(1))

	// Scaled to zero, the workload must still be reported as stopped.
	_, err = c.client.AppsV1().Deployments(d.Namespace).UpdateScale(ctx, d.Name, &autoscalingv1.Scale{
		ObjectMeta: metav1.ObjectMeta{Name: d.Name},
		Spec:       autoscalingv1.ScaleSpec{Replicas: 0},
	}, metav1.UpdateOptions{})
	assert.NilError(t, err)
	assert.NilError(t, WaitForDeploymentScale(ctx, c.client, "default", d.Name, 0))

	got, err = p.InstanceInspect(ctx, name)
	assert.NilError(t, err)
	assert.Equal(t, got.Status, sablier.InstanceStatusStopped)
}
