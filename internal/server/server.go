package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"

	"github.com/sablierapp/sablier/internal/api"
	"github.com/sablierapp/sablier/pkg/config"
)

func setupRouter(ctx context.Context, logger *slog.Logger, serverConf config.Server, tracingConf config.Tracing, s *api.ServeStrategy) *gin.Engine {
	r := gin.New()

	// OpenTelemetry span-per-request middleware. Uses the global TracerProvider,
	// which is a no-op when tracing is disabled.
	r.Use(otelgin.Middleware(tracingConf.ServiceName))
	r.Use(StructuredLogger(logger))
	r.Use(gin.Recovery())

	registerRoutes(ctx, r, serverConf, s)

	return r
}

// minDrainTimeout is the floor for the shutdown drain window, and drainGrace
// is added on top of the blocking strategy's default timeout so a request held
// right up to that timeout still has time to write its response.
const (
	minDrainTimeout = 15 * time.Second
	drainGrace      = 5 * time.Second
)

// Start runs the HTTP server until ctx is cancelled, then drains in-flight
// requests before returning. It returns nil after a clean shutdown, or the
// fatal serve error (e.g. the port is already in use) so the caller can
// terminate the process instead of running on without a listener.
func Start(ctx context.Context, logger *slog.Logger, serverConf config.Server, tracingConf config.Tracing, s *api.ServeStrategy) error {
	start := time.Now()

	if logger.Enabled(ctx, slog.LevelDebug) {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	r := setupRouter(ctx, logger, serverConf, tracingConf, s)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", serverConf.Port),
		Handler: r,
	}

	logger.Info("starting ",
		slog.String("listen", server.Addr),
		slog.Duration("startup", time.Since(start)),
		slog.String("mode", gin.Mode()),
	)

	errC := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errC <- err
		}
	}()

	select {
	case err := <-errC:
		// ListenAndServe failed outright (port in use, bad address, ...).
		return fmt.Errorf("server: %w", err)
	case <-ctx.Done():
	}

	// Drain in-flight requests instead of resetting them: the blocking
	// strategy holds requests open up to its configured timeout by design, so
	// the drain window must outlive it. The health endpoint already reports
	// 503 once ctx is cancelled, taking the instance out of rotation while it
	// drains.
	drain := drainTimeout(s.StrategyConfig.Blocking.DefaultTimeout)
	logger.Info("server: shutting down, draining in-flight requests", slog.Duration("drain_timeout", drain))
	shutdownCtx, cancel := context.WithTimeout(context.Background(), drain)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("server: drain did not complete in time, closing remaining connections", slog.Any("error", err))
		_ = server.Close()
		return nil
	}
	logger.Info("server: shutdown complete")
	return nil
}

// drainTimeout returns the shutdown drain window for the given blocking-
// strategy default timeout: the timeout plus a response grace, never below
// minDrainTimeout.
func drainTimeout(blockingTimeout time.Duration) time.Duration {
	if d := blockingTimeout + drainGrace; d > minDrainTimeout {
		return d
	}
	return minDrainTimeout
}
