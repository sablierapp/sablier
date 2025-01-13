package server

import (
	"github.com/gin-gonic/gin"
	"github.com/sablierapp/sablier/internal/api"
	"github.com/sablierapp/sablier/pkg/sablier"
)

func registerRoutes(router *gin.Engine, s *sablier.Sablier) {
	// Enables automatic redirection if the current route cannot be matched but a
	// handler for the path with (without) the trailing slash exists.
	router.RedirectTrailingSlash = true

	// Create REST API router group.
	APIv1 := router.Group("/api")

	api.StartBlocking(APIv1, s)
	api.StartDynamic(APIv1, s)
	api.GetGroups(APIv1, s)
	api.GetThemes(APIv1, s)
	api.PreviewTheme(APIv1, s)
	api.ListInstances(APIv1, s)
}
