package kubernetes

import (
	"context"
	"github.com/sablierapp/sablier/app/discovery"
	"github.com/sablierapp/sablier/app/types"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/apps/v1"
	core_v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"strconv"
)

const (
	LabelEnable                      = "sablierapp.dev/enable"
	LabelGroup                       = "sablierapp.dev/group"
	LabelGroupDefaultValue           = "default"
	LabelReplicas                    = "sablierapp.dev/replicas"
	LabelReplicasDefaultValue uint64 = 1
)

func (provider *KubernetesProvider) List(ctx context.Context) ([]types.Instance, error) {
	deployments, err := provider.deploymentList(ctx)
	if err != nil {
		return nil, err
	}

	statefulSets, err := provider.statefulSetList(ctx)
	if err != nil {
		return nil, err
	}

	return append(deployments, statefulSets...), nil
}

func (provider *KubernetesProvider) deploymentList(ctx context.Context) ([]types.Instance, error) {
	requirement, err := labels.NewRequirement(LabelEnable, selection.Equals, []string{"true"})
	if err != nil {
		return nil, err
	}
	requirementDeprecated, err := labels.NewRequirement(discovery.LabelEnable, selection.Equals, []string{"true"})
	if err != nil {
		return nil, err
	}
	selector := labels.NewSelector()
	selector = selector.Add(*requirement, *requirementDeprecated)
	deployments, err := provider.Client.AppsV1().Deployments(core_v1.NamespaceAll).List(ctx, metav1.ListOptions{
		LabelSelector: selector.String(),
	})

	if err != nil {
		return nil, err
	}

	instances := make([]types.Instance, 0, len(deployments.Items))
	for _, d := range deployments.Items {
		instance := provider.deploymentToInstance(d)
		instances = append(instances, instance)
	}

	return instances, nil
}

func (provider *KubernetesProvider) deploymentToInstance(d v1.Deployment) types.Instance {
	var group string
	var replicas uint64

	if _, ok := d.Labels[LabelEnable]; ok {
		if g, ok := d.Labels[LabelGroup]; ok {
			group = g
		} else {
			group = LabelGroupDefaultValue
		}

		if r, ok := d.Labels[LabelReplicas]; ok {
			atoi, err := strconv.Atoi(r)
			if err != nil {
				log.Warnf("Defaulting to default replicas value, could not convert value \"%v\" to int: %v", r, err)
				replicas = LabelReplicasDefaultValue
			} else {
				replicas = uint64(atoi)
			}
		} else {
			replicas = LabelReplicasDefaultValue
		}
	}

	parsed := DeploymentName(d, ParseOptions{Delimiter: provider.delimiter})

	return types.Instance{
		Name:            parsed.Original,
		Kind:            parsed.Kind,
		Status:          d.Status.String(),
		Replicas:        uint64(d.Status.Replicas),
		DesiredReplicas: uint64(*d.Spec.Replicas),
		ScalingReplicas: replicas,
		Group:           group,
	}
}

func (provider *KubernetesProvider) statefulSetList(ctx context.Context) ([]types.Instance, error) {
	requirement, err := labels.NewRequirement(LabelEnable, selection.Equals, []string{"true"})
	if err != nil {
		return nil, err
	}
	requirementDeprecated, err := labels.NewRequirement(discovery.LabelEnable, selection.Equals, []string{"true"})
	if err != nil {
		return nil, err
	}
	selector := labels.NewSelector()
	selector = selector.Add(*requirement, *requirementDeprecated)
	statefulSets, err := provider.Client.AppsV1().StatefulSets(core_v1.NamespaceAll).List(ctx, metav1.ListOptions{
		LabelSelector: selector.String(),
	})

	if err != nil {
		return nil, err
	}

	instances := make([]types.Instance, 0, len(statefulSets.Items))
	for _, ss := range statefulSets.Items {
		instance := provider.statefulSetToInstance(ss)
		instances = append(instances, instance)
	}

	return instances, nil
}

func (provider *KubernetesProvider) statefulSetToInstance(ss v1.StatefulSet) types.Instance {
	var group string
	var replicas uint64

	if _, ok := ss.Labels[LabelEnable]; ok {
		if g, ok := ss.Labels[LabelGroup]; ok {
			group = g
		} else {
			group = LabelGroupDefaultValue
		}

		if r, ok := ss.Labels[LabelReplicas]; ok {
			atoi, err := strconv.Atoi(r)
			if err != nil {
				log.Warnf("Defaulting to default replicas value, could not convert value \"%v\" to int: %v", r, err)
				replicas = LabelReplicasDefaultValue
			} else {
				replicas = uint64(atoi)
			}
		} else {
			replicas = LabelReplicasDefaultValue
		}
	}

	parsed := StatefulSetName(ss, ParseOptions{Delimiter: provider.delimiter})

	return types.Instance{
		Name:            parsed.Original,
		Kind:            parsed.Kind,
		Status:          ss.Status.String(),
		Replicas:        uint64(ss.Status.Replicas),
		DesiredReplicas: uint64(*ss.Spec.Replicas),
		ScalingReplicas: replicas,
		Group:           group,
	}
}
