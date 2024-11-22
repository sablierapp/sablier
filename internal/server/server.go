package server

import (
	"context"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"github.com/sablierapp/sablier/pkg/sablier"
	"net/http"
	"time"
)

func Start(ctx context.Context, s *sablier.Sablier) {
	start := time.Now()

	// Set web server mode.
	/*
		if conf.HttpMode() != "" {
			gin.SetMode(conf.HttpMode())
		} else if conf.Debug() == false {
			gin.SetMode(gin.ReleaseMode)
		}
	*/

	// Create new router engine without standard middleware.
	router := gin.New()

	router.Use(Recovery())

	registerRoutes(router, s)

	var server *http.Server
	server = &http.Server{
		Addr:    "0.0.0.0:10000",
		Handler: router,
	}

	log.Info().
		Str("listen", server.Addr).
		Dur("startup", time.Since(start))

	go StartHttp(server)

	// Graceful web server shutdown.
	<-ctx.Done()
	log.Info().Msg("server: shutting down")
	err := server.Close()
	if err != nil {
		log.Err(err).Msg("server: shutdown failed")
	}
}

// StartHttp starts the Web server in http mode.
func StartHttp(s *http.Server) {
	if err := s.ListenAndServe(); err != nil {
		if errors.Is(err, http.ErrServerClosed) {
			log.Info().Msg("server: shutdown complete")
		} else {
			log.Err(err).Msg("server: shutdown failed")
		}
	}
}
