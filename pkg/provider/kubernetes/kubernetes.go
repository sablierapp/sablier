package kubernetes

import (
	"context"
	"github.com/sablierapp/sablier/app/discovery"
	"github.com/sablierapp/sablier/pkg/provider"
	core_v1 "k8s.io/api/core/v1"
	"log/slog"

	providerConfig "github.com/sablierapp/sablier/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Interface guard
var _ provider.Provider = (*KubernetesProvider)(nil)

type KubernetesProvider struct {
	Client    kubernetes.Interface
	delimiter string
	l         *slog.Logger
}

func NewKubernetesProvider(ctx context.Context, client *kubernetes.Clientset, logger *slog.Logger, kubeclientConfig providerConfig.Kubernetes) (*KubernetesProvider, error) {
	logger = logger.With(slog.String("provider", "kubernetes"))

	info, err := client.ServerVersion()
	if err != nil {
		return nil, err
	}

	logger.InfoContext(ctx, "connection established with kubernetes",
		slog.String("version", info.String()),
		slog.Float64("config.qps", float64(kubeclientConfig.QPS)),
		slog.Int("config.burst", kubeclientConfig.Burst),
	)

	return &KubernetesProvider{
		Client:    client,
		delimiter: kubeclientConfig.Delimiter,
		l:         logger,
	}, nil

}

func (p *KubernetesProvider) Start(ctx context.Context, name string) error {
	parsed, err := ParseName(name, ParseOptions{Delimiter: p.delimiter})
	if err != nil {
		return err
	}

	return p.scale(ctx, parsed, parsed.Replicas)
}

func (p *KubernetesProvider) Stop(ctx context.Context, name string) error {
	parsed, err := ParseName(name, ParseOptions{Delimiter: p.delimiter})
	if err != nil {
		return err
	}

	return p.scale(ctx, parsed, 0)
}

func (p *KubernetesProvider) GetGroups(ctx context.Context) (map[string][]string, error) {
	deployments, err := p.Client.AppsV1().Deployments(core_v1.NamespaceAll).List(ctx, metav1.ListOptions{
		LabelSelector: discovery.LabelEnable,
	})

	if err != nil {
		return nil, err
	}

	groups := make(map[string][]string)
	for _, deployment := range deployments.Items {
		groupName := deployment.Labels[discovery.LabelGroup]
		if len(groupName) == 0 {
			groupName = discovery.LabelGroupDefaultValue
		}

		group := groups[groupName]
		parsed := DeploymentName(&deployment, ParseOptions{Delimiter: p.delimiter})
		group = append(group, parsed.Original)
		groups[groupName] = group
	}

	statefulSets, err := p.Client.AppsV1().StatefulSets(core_v1.NamespaceAll).List(ctx, metav1.ListOptions{
		LabelSelector: discovery.LabelEnable,
	})

	if err != nil {
		return nil, err
	}

	for _, statefulSet := range statefulSets.Items {
		groupName := statefulSet.Labels[discovery.LabelGroup]
		if len(groupName) == 0 {
			groupName = discovery.LabelGroupDefaultValue
		}

		group := groups[groupName]
		parsed := StatefulSetName(&statefulSet, ParseOptions{Delimiter: p.delimiter})
		group = append(group, parsed.Original)
		groups[groupName] = group
	}

	return groups, nil
}
