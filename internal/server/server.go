package server

import (
	"context"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/sablierapp/sablier/pkg/sablier"
	"net/http"
	"time"
)

func Start(ctx context.Context, s *sablier.Sablier, log zerolog.Logger) {
	start := time.Now()

	// Set web server mode.
	/*
		if conf.HttpMode() != "" {
			gin.SetMode(conf.HttpMode())
		} else if conf.Debug() == false {
			gin.SetMode(gin.ReleaseMode)
		}
	*/

	// Create new r engine without standard middleware.
	r := gin.New()

	r.Use(StructuredLogger(log))
	r.Use(Recovery())

	registerRoutes(r, s)

	var server *http.Server
	server = &http.Server{
		Addr:    "0.0.0.0:10000",
		Handler: r,
	}

	log.Info().
		Str("listen", server.Addr).
		Dur("startup", time.Since(start))

	go StartHttp(server, log)

	// Graceful web server shutdown.
	<-ctx.Done()
	log.Info().Msg("server: shutting down")
	err := server.Close()
	if err != nil {
		log.Err(err).Msg("server: shutdown failed")
	}
}

// StartHttp starts the Web server in http mode.
func StartHttp(s *http.Server, log zerolog.Logger) {
	if err := s.ListenAndServe(); err != nil {
		if errors.Is(err, http.ErrServerClosed) {
			log.Info().Msg("server: shutdown complete")
		} else {
			log.Err(err).Msg("server: shutdown failed")
		}
	}
}
