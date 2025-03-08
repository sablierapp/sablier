package kubernetes

import (
	"context"
	"github.com/sablierapp/sablier/pkg/sablier"
	"log/slog"

	providerConfig "github.com/sablierapp/sablier/config"
	"k8s.io/client-go/kubernetes"
)

// Interface guard
var _ sablier.Provider = (*KubernetesProvider)(nil)

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
