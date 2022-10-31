package http

import (
	"fmt"

	log "github.com/sirupsen/logrus"

	"github.com/acouvreur/sablier/app/http/middleware"
	"github.com/acouvreur/sablier/app/http/routes"
	"github.com/acouvreur/sablier/app/sessions"
	"github.com/acouvreur/sablier/config"
	"github.com/gin-gonic/gin"
)

func Start(serverConf config.Server, strategyConf config.Strategy, sessionManager sessions.Manager) error {
	r := gin.New()

	r.Use(middleware.Logger(log.New()), gin.Recovery())

	base := r.Group(serverConf.BasePath)
	{
		api := base.Group("/api")
		{
			strategy := routes.NewServeStrategy(sessionManager, strategyConf)
			api.GET("/strategies/dynamic", strategy.ServeDynamic)
			api.GET("/strategies/blocking", strategy.ServeBlocking)
		}
	}

	logRoutes(r.Routes())

	return r.Run(fmt.Sprintf(":%d", serverConf.Port))
}

func logRoutes(routes gin.RoutesInfo) {
	for _, route := range routes {
		log.Info(fmt.Sprintf("%s %s %s", route.Method, route.Path, route.Handler))
	}
}
