package kubernetes

import (
	"context"
	"log/slog"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"k8s.io/client-go/dynamic"
	k8s "k8s.io/client-go/kubernetes"

	providerConfig "github.com/sablierapp/sablier/pkg/config"
	"github.com/sablierapp/sablier/pkg/sablier"
)

// Interface guard
var _ sablier.Provider = (*Provider)(nil)

type Provider struct {
	Client k8s.Interface
	// dynamic drives Custom Resources (e.g. CloudNativePG Clusters) that are not
	// part of the typed clientset. It may be nil when the provider is constructed
	// without CRD support; CRD-backed operations guard against that.
	dynamic             dynamic.Interface
	delimiter           string
	readyOnFirstReplica bool
	l                   *slog.Logger
	tracer              trace.Tracer
	// scaleFallbackWarned holds the kinds for which scale already warned that it
	// fell back to the deprecated GET + UPDATE.
	scaleFallbackWarned sync.Map
	// clusterListForbiddenWarned ensures the CNPG list Forbidden skip warns only once.
	clusterListForbiddenWarned sync.Once
}

func New(ctx context.Context, client *k8s.Clientset, dynamicClient dynamic.Interface, logger *slog.Logger, config providerConfig.Kubernetes) (*Provider, error) {
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
		Client:              client,
		dynamic:             dynamicClient,
		delimiter:           config.Delimiter,
		readyOnFirstReplica: config.ReadyOnFirstReplica,
		l:                   logger,
		tracer:              otel.Tracer("github.com/sablierapp/sablier/pkg/provider/kubernetes"),
	}, nil

}

// warnClusterListForbidden logs once that CloudNativePG Clusters are skipped
// because the CRD is installed but listing it is not granted.
func (p *Provider) warnClusterListForbidden(ctx context.Context, err error) {
	p.clusterListForbiddenWarned.Do(func() {
		p.l.WarnContext(ctx, "listing cloudnativepg clusters is forbidden, skipping cnpg cluster discovery. "+
			"Grant get, list and watch on clusters.postgresql.cnpg.io in Sablier's ClusterRole to manage CloudNativePG Clusters.",
			slog.Any("error", err))
	})
}
