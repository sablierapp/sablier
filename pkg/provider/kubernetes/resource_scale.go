package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// buildResourcePatch creates a strategic-merge-patch JSON document that sets
// the resource limits on the first container (identified by containerName) of a
// pod template. Only the fields present in cpu/memory are included.
func buildResourcePatch(containerName, cpu, memory string) ([]byte, error) {
	limits := corev1.ResourceList{}

	if cpu != "" {
		q, err := resource.ParseQuantity(cpu)
		if err != nil {
			return nil, fmt.Errorf("invalid CPU value %q: %w", cpu, err)
		}
		limits[corev1.ResourceCPU] = q
	}

	if memory != "" {
		q, err := resource.ParseQuantity(memory)
		if err != nil {
			return nil, fmt.Errorf("invalid memory value %q: %w", memory, err)
		}
		limits[corev1.ResourceMemory] = q
	}

	patch := map[string]interface{}{
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []map[string]interface{}{
						{
							"name": containerName,
							"resources": map[string]interface{}{
								"limits": limits,
							},
						},
					},
				},
			},
		},
	}

	return json.Marshal(patch)
}

// scaleResources patches the resource limits of the first container in a
// deployment or statefulset without changing the replica count.
func (p *Provider) scaleResources(ctx context.Context, config ParsedName, cpu, memory string) error {
	switch config.Kind {
	case "deployment":
		return p.scaleDeploymentResources(ctx, config, cpu, memory)
	case "statefulset":
		return p.scaleStatefulSetResources(ctx, config, cpu, memory)
	default:
		return fmt.Errorf("unsupported kind %q for resource scaling", config.Kind)
	}
}

func (p *Provider) scaleDeploymentResources(ctx context.Context, config ParsedName, cpu, memory string) error {
	d, err := p.Client.AppsV1().Deployments(config.Namespace).Get(ctx, config.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("cannot get deployment: %w", err)
	}

	if len(d.Spec.Template.Spec.Containers) == 0 {
		return fmt.Errorf("deployment %s/%s has no containers", config.Namespace, config.Name)
	}

	patchData, err := buildResourcePatch(d.Spec.Template.Spec.Containers[0].Name, cpu, memory)
	if err != nil {
		return err
	}

	_, err = p.Client.AppsV1().Deployments(config.Namespace).Patch(
		ctx, config.Name, types.StrategicMergePatchType, patchData, metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("cannot patch deployment resources: %w", err)
	}
	return nil
}

func (p *Provider) scaleStatefulSetResources(ctx context.Context, config ParsedName, cpu, memory string) error {
	ss, err := p.Client.AppsV1().StatefulSets(config.Namespace).Get(ctx, config.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("cannot get statefulset: %w", err)
	}

	if len(ss.Spec.Template.Spec.Containers) == 0 {
		return fmt.Errorf("statefulset %s/%s has no containers", config.Namespace, config.Name)
	}

	patchData, err := buildResourcePatch(ss.Spec.Template.Spec.Containers[0].Name, cpu, memory)
	if err != nil {
		return err
	}

	_, err = p.Client.AppsV1().StatefulSets(config.Namespace).Patch(
		ctx, config.Name, types.StrategicMergePatchType, patchData, metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("cannot patch statefulset resources: %w", err)
	}
	return nil
}

// getWorkloadLabels retrieves the labels of a deployment or statefulset.
func (p *Provider) getWorkloadLabels(ctx context.Context, config ParsedName) (map[string]string, error) {
	switch config.Kind {
	case "deployment":
		d, err := p.Client.AppsV1().Deployments(config.Namespace).Get(ctx, config.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		if d.Labels == nil {
			return map[string]string{}, nil
		}
		return d.Labels, nil
	case "statefulset":
		ss, err := p.Client.AppsV1().StatefulSets(config.Namespace).Get(ctx, config.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		if ss.Labels == nil {
			return map[string]string{}, nil
		}
		return ss.Labels, nil
	default:
		return nil, fmt.Errorf("unsupported kind %q", config.Kind)
	}
}
