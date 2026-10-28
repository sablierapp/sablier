package server

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/sablierapp/sablier/internal/api"
	"github.com/sablierapp/sablier/pkg/config"
	"github.com/sablierapp/sablier/pkg/metrics"
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
	api.ListThemes(APIv1, s)
	// Everything that touches sessions or workloads is served by the leader only.
	leaderOnly := APIv1.Group("", api.RequireLeader(s))
	api.StartDynamic(leaderOnly, s)
	api.StartBlocking(leaderOnly, s)
	api.StartPoke(leaderOnly, s)
	api.InstanceEvents(leaderOnly, s)
}
