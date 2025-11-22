package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sablierapp/sablier/internal/api"
	"github.com/sablierapp/sablier/pkg/config"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

func setupRouter(ctx context.Context, logger *slog.Logger, serverConf config.Server, s *api.ServeStrategy) *gin.Engine {
	r := gin.New()

	r.Use(StructuredLogger(logger))
	r.Use(gin.Recovery())
	r.Use(otelgin.Middleware("sablier"))

	registerRoutes(ctx, r, serverConf, s)

	return r
}

func Start(ctx context.Context, logger *slog.Logger, serverConf config.Server, s *api.ServeStrategy) {
	start := time.Now()

	if logger.Enabled(ctx, slog.LevelDebug) {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	r := setupRouter(ctx, logger, serverConf, s)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", serverConf.Port),
		Handler: r,
	}

	logger.Info("starting ",
		slog.String("listen", server.Addr),
		slog.Duration("startup", time.Since(start)),
		slog.String("mode", gin.Mode()),
	)

	go StartHttp(server, logger)

	// Graceful web server shutdown.
	<-ctx.Done()
	logger.Info("server: shutting down")
	err := server.Close()
	if err != nil {
		logger.Error("server: shutdown failed", slog.Any("error", err))
	}
}

// StartHttp starts the Web server in http mode.
func StartHttp(s *http.Server, logger *slog.Logger) {
	if err := s.ListenAndServe(); err != nil {
		if errors.Is(err, http.ErrServerClosed) {
			logger.Info("server: shutdown complete")
		} else {
			logger.Error("server failed to start", slog.Any("error", err))
		}
	}
}
