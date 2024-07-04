package kubernetes

import (
	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func TestParseName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		opts     ParseOptions
		expected ParsedName
		hasError bool
	}{
		{
			name:     "Valid name with default delimiter",
			input:    "deployment:namespace:name",
			opts:     ParseOptions{Delimiter: ":"},
			expected: ParsedName{Original: "deployment:namespace:name", Kind: "deployment", Namespace: "namespace", Name: "name"},
			hasError: false,
		},
		{
			name:     "Invalid name with missing parts",
			input:    "deployment:namespace",
			opts:     ParseOptions{Delimiter: ":"},
			expected: ParsedName{},
			hasError: true,
		},
		{
			name:     "Valid name with custom delimiter",
			input:    "statefulset#namespace#name",
			opts:     ParseOptions{Delimiter: "#"},
			expected: ParsedName{Original: "statefulset#namespace#name", Kind: "statefulset", Namespace: "namespace", Name: "name"},
			hasError: false,
		},
		{
			name:     "Invalid name with incorrect delimiter",
			input:    "statefulset:namespace:name",
			opts:     ParseOptions{Delimiter: "#"},
			expected: ParsedName{},
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseName(tt.input, tt.opts)
			if tt.hasError {
				if err == nil {
					t.Errorf("expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("expected no error but got %v", err)
				}
				if result != tt.expected {
					t.Errorf("expected %v but got %v", tt.expected, result)
				}
			}
		})
	}
}

func TestDeploymentName(t *testing.T) {
	deployment := v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-namespace",
			Name:      "test-deployment",
		},
	}
	opts := ParseOptions{Delimiter: ":"}
	expected := ParsedName{
		Original:  "deployment:test-namespace:test-deployment:1",
		Kind:      "deployment",
		Namespace: "test-namespace",
		Name:      "test-deployment",
	}

	result := DeploymentName(deployment, opts)
	if result != expected {
		t.Errorf("expected %v but got %v", expected, result)
	}
}

func TestStatefulSetName(t *testing.T) {
	statefulSet := v1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-namespace",
			Name:      "test-statefulset",
		},
	}
	opts := ParseOptions{Delimiter: ":"}
	expected := ParsedName{
		Original:  "statefulset:test-namespace:test-statefulset:1",
		Kind:      "statefulset",
		Namespace: "test-namespace",
		Name:      "test-statefulset",
	}

	result := StatefulSetName(statefulSet, opts)
	if result != expected {
		t.Errorf("expected %v but got %v", expected, result)
	}
}
