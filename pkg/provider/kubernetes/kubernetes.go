package kubernetes

import (
	"context"
	"github.com/sablierapp/sablier/pkg/sablier"
	"log/slog"

	providerConfig "github.com/sablierapp/sablier/config"
	"k8s.io/client-go/kubernetes"
)

// Interface guard
var _ sablier.Provider = (*Provider)(nil)

type Provider struct {
	Client    kubernetes.Interface
	delimiter string
	l         *slog.Logger
}

func New(ctx context.Context, client *kubernetes.Clientset, logger *slog.Logger, config providerConfig.Kubernetes) (*Provider, error) {
	logger = logger.With(slog.String("provider", "kubernetes"))

	info, err := client.ServerVersion()
	if err != nil {
		return nil, err
	}

	logger.InfoContext(ctx, "connection established with kubernetes",
		slog.String("version", info.String()),
		slog.Float64("config.qps", float64(config.QPS)),
		slog.Int("config.burst", config.Burst),
	)

	return &Provider{
		Client:    client,
		delimiter: config.Delimiter,
		l:         logger,
	}, nil

}
