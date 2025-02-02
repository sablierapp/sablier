package server

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/sablierapp/sablier/app/http/routes"
	"github.com/sablierapp/sablier/config"
	"github.com/sablierapp/sablier/internal/api"
)

func registerRoutes(ctx context.Context, router *gin.Engine, serverConf config.Server, s *routes.ServeStrategy) {
	// Enables automatic redirection if the current route cannot be matched but a
	// handler for the path with (without) the trailing slash exists.
	router.RedirectTrailingSlash = true

	base := router.Group(serverConf.BasePath)

	api.Healthcheck(base, ctx)

	// Create REST API router group.
	APIv1 := base.Group("/api")

	api.StartDynamic(APIv1, s)
	api.StartBlocking(APIv1, s)
	api.ListThemes(APIv1, s)
}
