package kubernetes

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	k8s "k8s.io/client-go/kubernetes"

	providerConfig "github.com/sablierapp/sablier/pkg/config"
	"github.com/sablierapp/sablier/pkg/sablier"
)

// Interface guard
var _ sablier.Provider = (*Provider)(nil)

type Provider struct {
	Client    k8s.Interface
	delimiter string
	l         *slog.Logger
	tracer    trace.Tracer
}

func New(ctx context.Context, client *k8s.Clientset, logger *slog.Logger, config providerConfig.Kubernetes) (*Provider, error) {
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
		tracer:    otel.Tracer("github.com/sablierapp/sablier/pkg/provider/kubernetes"),
	}, nil

}
