package http

import (
	"context"
	"fmt"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/acouvreur/sablier/app/http/middleware"
	"github.com/acouvreur/sablier/app/http/routes"
	"github.com/acouvreur/sablier/app/sessions"
	"github.com/acouvreur/sablier/config"
	"github.com/gin-gonic/gin"
)

func Start(serverConf config.Server, strategyConf config.Strategy, sessionManager sessions.Manager) {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	r := gin.New()

	r.Use(middleware.Logger(log.New()), gin.Recovery())

	base := r.Group(serverConf.BasePath)
	{
		api := base.Group("/api")
		{
			strategy := routes.NewServeStrategy(sessionManager, strategyConf)
			api.GET("/strategies/dynamic", strategy.ServeDynamic)
			api.GET("/strategies/dynamic/themes", strategy.ServeDynamicThemes)
			api.GET("/strategies/blocking", strategy.ServeBlocking)
		}
	}

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", serverConf.Port),
		Handler: r,
	}

	// Initializing the server in a goroutine so that
	// it won't block the graceful shutdown handling below
	go func() {
		log.Info("server listening ", srv.Addr)
		logRoutes(r.Routes())
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	// Listen for the interrupt signal.
	<-ctx.Done()

	// Restore default behavior on the interrupt signal and notify user of shutdown.
	stop()
	log.Info("shutting down gracefully, press Ctrl+C again to force")

	// The context is used to inform the server it has 10 seconds to finish
	// the request it is currently handling
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("server forced to shutdown: ", err)
	}

	log.Info("server exiting")

}

func logRoutes(routes gin.RoutesInfo) {
	for _, route := range routes {
		log.Info(fmt.Sprintf("%s %s %s", route.Method, route.Path, route.Handler))
	}
}
