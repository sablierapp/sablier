package server

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/sablierapp/sablier/internal/api"
	"github.com/sablierapp/sablier/internal/dashboard"
	"github.com/sablierapp/sablier/pkg/config"
	"github.com/sablierapp/sablier/pkg/metrics"
	"github.com/sablierapp/sablier/pkg/sablier"
)

func registerRoutes(ctx context.Context, router *gin.Engine, serverConf config.Server, s *api.ServeStrategy) {
	router.RedirectTrailingSlash = true

	base := router.Group(serverConf.BasePath)

	api.Healthcheck(base, ctx)

	// Register /metrics only when a real PromRecorder is in use.
	if rec, ok := s.Metrics.(*metrics.PromRecorder); ok {
		base.GET("/metrics", gin.WrapH(metrics.NewHandler(rec)))
	}

	APIv1 := base.Group("/api")
	api.StartDynamic(APIv1, s)
	api.StartBlocking(APIv1, s)
	api.ListThemes(APIv1, s)
	api.InstanceEvents(APIv1, s)

	var snapProvider dashboard.SnapshotProvider
	if sp, ok := s.Sablier.(interface {
		SnapshotSessions(ctx context.Context) ([]sablier.InstanceInfo, error)
		Groups() map[string][]string
	}); ok {
		snapProvider = sp
	}

	dashHandler := &dashboard.Handler{
		Sablier:        snapProvider,
		Metrics:        s.Metrics,
		ProviderConfig: s.ProviderConfig,
		SessionConfig:  s.SessionsConfig,
	}
	dashboard.Register(base.Group("/dashboard"), dashHandler)
}
