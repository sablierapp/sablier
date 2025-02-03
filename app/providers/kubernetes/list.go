package kubernetes

import (
	"context"
	"github.com/sablierapp/sablier/app/discovery"
	"github.com/sablierapp/sablier/app/providers"
	"github.com/sablierapp/sablier/app/types"
	v1 "k8s.io/api/apps/v1"
	core_v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"log/slog"
	"strconv"
	"strings"
)

func (p *KubernetesProvider) InstanceList(ctx context.Context, options providers.InstanceListOptions) ([]types.Instance, error) {
	deployments, err := p.deploymentList(ctx, options)
	if err != nil {
		return nil, err
	}

	statefulSets, err := p.statefulSetList(ctx, options)
	if err != nil {
		return nil, err
	}

	return append(deployments, statefulSets...), nil
}

func (p *KubernetesProvider) deploymentList(ctx context.Context, options providers.InstanceListOptions) ([]types.Instance, error) {
	deployments, err := p.Client.AppsV1().Deployments(core_v1.NamespaceAll).List(ctx, metav1.ListOptions{
		LabelSelector: strings.Join(options.Labels, ","),
	})

	if err != nil {
		return nil, err
	}

	instances := make([]types.Instance, 0, len(deployments.Items))
	for _, d := range deployments.Items {
		instance := p.deploymentToInstance(d)
		instances = append(instances, instance)
	}

	return instances, nil
}

func (p *KubernetesProvider) deploymentToInstance(d v1.Deployment) types.Instance {
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
				p.l.Warn("invalid replicas label value, using default replicas value", slog.Any("error", err), slog.String("instance", d.Name), slog.String("value", r))
				replicas = discovery.LabelReplicasDefaultValue
			} else {
				replicas = uint64(atoi)
			}
		} else {
			replicas = discovery.LabelReplicasDefaultValue
		}
	}

	parsed := DeploymentName(d, ParseOptions{Delimiter: p.delimiter})

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

func (p *KubernetesProvider) statefulSetList(ctx context.Context, options providers.InstanceListOptions) ([]types.Instance, error) {
	statefulSets, err := p.Client.AppsV1().StatefulSets(core_v1.NamespaceAll).List(ctx, metav1.ListOptions{
		LabelSelector: strings.Join(options.Labels, ","),
	})

	if err != nil {
		return nil, err
	}

	instances := make([]types.Instance, 0, len(statefulSets.Items))
	for _, ss := range statefulSets.Items {
		instance := p.statefulSetToInstance(ss)
		instances = append(instances, instance)
	}

	return instances, nil
}

func (p *KubernetesProvider) statefulSetToInstance(ss v1.StatefulSet) types.Instance {
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
				p.l.Warn("invalid replicas label value, using default replicas value", slog.Any("error", err), slog.String("instance", ss.Name), slog.String("value", r))
				replicas = discovery.LabelReplicasDefaultValue
			} else {
				replicas = uint64(atoi)
			}
		} else {
			replicas = discovery.LabelReplicasDefaultValue
		}
	}

	parsed := StatefulSetName(ss, ParseOptions{Delimiter: p.delimiter})

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
