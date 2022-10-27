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

func Start(conf config.Server, sessionManager sessions.Manager) error {
	r := gin.New()

	r.Use(middleware.Logger(log.New()), gin.Recovery())

	base := r.Group(conf.BasePath)
	{
		api := base.Group("/api")
		{
			strategy := routes.ServeStrategy{SessionsManager: sessionManager}
			api.GET("/strategies/dynamic", strategy.ServeDynamic)
			api.GET("/strategies/blocking", strategy.ServeBlocking)
		}
	}

	logRoutes(r.Routes())

	return r.Run(fmt.Sprintf(":%d", conf.Port))
}

func logRoutes(routes gin.RoutesInfo) {
	for _, route := range routes {
		log.Info(fmt.Sprintf("%s %s %s", route.Method, route.Path, route.Handler))
	}
}
