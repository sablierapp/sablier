package kubernetes

import (
	"context"
	"github.com/acouvreur/sablier/app/discovery"
	"github.com/acouvreur/sablier/app/providers"
	"github.com/acouvreur/sablier/app/types"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/apps/v1"
	core_v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strconv"
	"strings"
)

func (provider *KubernetesProvider) InstanceList(ctx context.Context, options providers.InstanceListOptions) ([]types.Instance, error) {
	deployments, err := provider.deploymentList(ctx, options)
	if err != nil {
		return nil, err
	}

	statefulSets, err := provider.statefulSetList(ctx, options)
	if err != nil {
		return nil, err
	}

	return append(deployments, statefulSets...), nil
}

func (provider *KubernetesProvider) deploymentList(ctx context.Context, options providers.InstanceListOptions) ([]types.Instance, error) {
	deployments, err := provider.Client.AppsV1().Deployments(core_v1.NamespaceAll).List(ctx, metav1.ListOptions{
		LabelSelector: strings.Join(options.Labels, ","),
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

	if _, ok := d.Labels[discovery.LabelEnable]; ok {
		if g, ok := d.Labels[discovery.LabelGroup]; ok {
			group = g
		} else {
			group = discovery.LabelGroupDefaultValue
		}

		if r, ok := d.Labels[discovery.LabelReplicas]; ok {
			atoi, err := strconv.Atoi(r)
			if err != nil {
				log.Warnf("Defaulting to default replicas value, could not convert value \"%v\" to int: %v", r, err)
				replicas = discovery.LabelReplicasDefaultValue
			} else {
				replicas = uint64(atoi)
			}
		} else {
			replicas = discovery.LabelReplicasDefaultValue
		}
	}

	parsed := DeploymentName(d, ParseOptions{Delimiter: provider.delimiter})

	return types.Instance{
		Name:            parsed.Name,
		Kind:            parsed.Kind,
		Status:          d.Status.String(),
		Replicas:        uint64(d.Status.Replicas),
		DesiredReplicas: uint64(*d.Spec.Replicas),
		ScalingReplicas: replicas,
		Group:           group,
	}
}

func (provider *KubernetesProvider) statefulSetList(ctx context.Context, options providers.InstanceListOptions) ([]types.Instance, error) {
	statefulSets, err := provider.Client.AppsV1().StatefulSets(core_v1.NamespaceAll).List(ctx, metav1.ListOptions{
		LabelSelector: strings.Join(options.Labels, ","),
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

	if _, ok := ss.Labels[discovery.LabelEnable]; ok {
		if g, ok := ss.Labels[discovery.LabelGroup]; ok {
			group = g
		} else {
			group = discovery.LabelGroupDefaultValue
		}

		if r, ok := ss.Labels[discovery.LabelReplicas]; ok {
			atoi, err := strconv.Atoi(r)
			if err != nil {
				log.Warnf("Defaulting to default replicas value, could not convert value \"%v\" to int: %v", r, err)
				replicas = discovery.LabelReplicasDefaultValue
			} else {
				replicas = uint64(atoi)
			}
		} else {
			replicas = discovery.LabelReplicasDefaultValue
		}
	}

	parsed := StatefulSetName(ss, ParseOptions{Delimiter: provider.delimiter})

	return types.Instance{
		Name:            parsed.Name,
		Kind:            parsed.Kind,
		Status:          ss.Status.String(),
		Replicas:        uint64(ss.Status.Replicas),
		DesiredReplicas: uint64(*ss.Spec.Replicas),
		ScalingReplicas: replicas,
		Group:           group,
	}
}
