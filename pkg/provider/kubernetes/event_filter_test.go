package kubernetes

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestProvider_IgnoreDeploymentEvent(t *testing.T) {
	tests := []struct {
		name            string
		ignoreUnlabeled bool
		labels          map[string]string
		wantIgnore      bool
	}{
		{
			name:            "ignore unlabeled disabled",
			ignoreUnlabeled: false,
			wantIgnore:      false,
		},
		{
			name:            "unlabeled deployment ignored",
			ignoreUnlabeled: true,
			wantIgnore:      true,
		},
		{
			name:            "managed deployment handled",
			ignoreUnlabeled: true,
			labels:          map[string]string{"sablier.enable": "true"},
			wantIgnore:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Provider{ignoreUnlabeled: tt.ignoreUnlabeled}
			d := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Labels: tt.labels},
			}

			if got := p.ignoreDeploymentEvent(d); got != tt.wantIgnore {
				t.Fatalf("ignoreDeploymentEvent() = %v, want %v", got, tt.wantIgnore)
			}
		})
	}
}

func TestProvider_IgnoreStatefulSetEvent(t *testing.T) {
	tests := []struct {
		name            string
		ignoreUnlabeled bool
		labels          map[string]string
		wantIgnore      bool
	}{
		{
			name:            "ignore unlabeled disabled",
			ignoreUnlabeled: false,
			wantIgnore:      false,
		},
		{
			name:            "unlabeled statefulset ignored",
			ignoreUnlabeled: true,
			wantIgnore:      true,
		},
		{
			name:            "managed statefulset handled",
			ignoreUnlabeled: true,
			labels:          map[string]string{"sablier.enable": "true"},
			wantIgnore:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Provider{ignoreUnlabeled: tt.ignoreUnlabeled}
			ss := &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{Labels: tt.labels},
			}

			if got := p.ignoreStatefulSetEvent(ss); got != tt.wantIgnore {
				t.Fatalf("ignoreStatefulSetEvent() = %v, want %v", got, tt.wantIgnore)
			}
		})
	}
}
