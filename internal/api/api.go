package api

import (
	"context"
	"errors"
	"fmt"
	"github.com/acouvreur/sablier/config"
	"github.com/acouvreur/sablier/internal/provider"
	"github.com/acouvreur/sablier/internal/session"
	"github.com/acouvreur/sablier/internal/theme"
	"github.com/gin-gonic/gin"
	"net/http"
	"os/signal"
	"path"
	"syscall"
	"time"
)

const (
	versionPath                       = "/version"
	healthcheckPath                   = "/health"
	sessionRequestBlockingByNamesPath = "/sessions-blocking-by-names"
	sessionRequestBlockingByGroupPath = "/sessions-blocking-by-group"
	sessionRequestDynamicByNamesPath  = "/sessions-dynamic-by-names"
	sessionRequestDynamicByGroupPath  = "/sessions-dynamic-by-group"
)

func Start(serverConf config.Server, strategyConf config.Strategy, sessionsConf config.Sessions, t theme.Themes, sm session.SessionManager, d provider.Discovery) {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	r := gin.New()
	r.Use(applyServerHeader)

	// r.Use(Logger(log.New()), gin.Recovery())

	base := r.Group(path.Join(serverConf.BasePath, "/api"))
	ServeHealthcheck(base, ctx)
	ServeVersion(base)

	rbs := RequestBlockingSession{
		defaults: BlockingSessionRequestDefaults{
			SessionDuration: sessionsConf.DefaultDuration,
			Timeout:         strategyConf.Blocking.DefaultTimeout,
			DesiredReplicas: 1,
		},
		session:   sm,
		discovery: d,
	}
	ServeSessionRequestBlocking(base, rbs)

	rds := RequestDynamicSession{
		defaults: DynamicSessionRequestDefaults{
			SessionDuration: sessionsConf.DefaultDuration,
			Theme:           strategyConf.Dynamic.DefaultTheme,
			ThemeOptions: DynamicRequestThemeOptions{
				Title:            "Sablier",
				DisplayName:      "your app",
				ShowDetails:      strategyConf.Dynamic.ShowDetailsByDefault,
				RefreshFrequency: strategyConf.Dynamic.DefaultRefreshFrequency,
			},
			DesiredReplicas: 1,
		},
		theme:     t,
		session:   sm,
		discovery: d,
	}
	ServeSessionRequestDynamic(base, rds)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", serverConf.Port),
		Handler: r,
	}

	// Initializing the server in a goroutine so that
	// it won't block the graceful shutdown handling below
	go func() {
		log.Info("server listening ", srv.Addr)
		logRoutes(r.Routes())
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
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

func ServeSessionRequestBlocking(group *gin.RouterGroup, rbs RequestBlockingSession) {
	group.POST(sessionRequestBlockingByGroupPath, rbs.RequestBlockingByGroup)
	group.POST(sessionRequestBlockingByNamesPath, rbs.RequestBlockingByNames)
}

func ServeSessionRequestDynamic(group *gin.RouterGroup, rds RequestDynamicSession) {
	group.POST(sessionRequestDynamicByGroupPath, rds.RequestDynamicByGroup)
	group.POST(sessionRequestDynamicByNamesPath, rds.RequestDynamicByNames)
}

func ServeVersion(group *gin.RouterGroup) {
	group.GET(versionPath, GetVersion)
}

func ServeHealthcheck(group *gin.RouterGroup, ctx context.Context) {
	health := Health{}
	health.SetDefaults()
	health.WithContext(ctx)
	group.GET(healthcheckPath, health.ServeHTTP)
}

func logRoutes(routes gin.RoutesInfo) {
	for _, route := range routes {
		log.Debug(fmt.Sprintf("%s %s %s", route.Method, route.Path, route.Handler))
	}
}
