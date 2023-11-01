package api

import (
	"context"
	"errors"
	"fmt"
	"github.com/acouvreur/sablier/config"
	"github.com/acouvreur/sablier/internal/provider"
	"github.com/acouvreur/sablier/internal/session"
	"github.com/acouvreur/sablier/internal/theme"
	"github.com/acouvreur/sablier/pkg/durations"
	"github.com/gin-gonic/gin"
	log "log/slog"
	"net/http"
	"path"
	"time"
)

const (
	versionPath                       = "/version"
	healthcheckPath                   = "/health"
	listThemesPath                    = "/themes"
	previewThemePath                  = "/themes/:theme"
	sessionRequestBlockingByNamesPath = "/sessions-blocking-by-names"
	sessionRequestBlockingByGroupPath = "/sessions-blocking-by-group"
	sessionRequestDynamicByNamesPath  = "/sessions-dynamic-by-names"
	sessionRequestDynamicByGroupPath  = "/sessions-dynamic-by-group"
	sessionsListPath                  = "/sessions"
	groupsListPath                    = "/groups"
)

func Start(ctx context.Context, conf config.Config, t *theme.Themes, sm *session.Manager, d *provider.Discovery) {
	r := gin.New()
	gin.EnableJsonDecoderDisallowUnknownFields()
	gin.EnableJsonDecoderUseNumber()

	r.Use(applyServerHeader)

	// r.Use(Logger(log.New()), gin.Recovery())

	base := r.Group(path.Join(conf.Server.BasePath, "/api"))
	ServeHealthcheck(base, ctx)
	ServeVersion(base)

	rbs := RequestBlockingSession{
		defaults: BlockingSessionRequestDefaults{
			SessionDuration: durations.Duration{Duration: conf.Sessions.DefaultDuration},
			Timeout:         durations.Duration{Duration: conf.Strategy.Blocking.DefaultTimeout},
			DesiredReplicas: 1,
		},
		session:   sm,
		discovery: d,
	}
	ServeSessionRequestBlocking(base, rbs)

	rds := RequestDynamicSession{
		defaults: DynamicSessionRequestDefaults{
			SessionDuration: durations.Duration{Duration: conf.Sessions.DefaultDuration},
			Theme:           conf.Strategy.Dynamic.DefaultTheme,
			ThemeOptions: DynamicRequestThemeOptions{
				Title:            "Sablier",
				DisplayName:      "your app",
				ShowDetails:      conf.Strategy.Dynamic.ShowDetailsByDefault,
				RefreshFrequency: durations.Duration{Duration: conf.Strategy.Dynamic.DefaultRefreshFrequency},
			},
			DesiredReplicas: 1,
		},
		theme:     t,
		session:   sm,
		discovery: d,
	}
	ServeSessionRequestDynamic(base, rds)
	ServeThemes(base, t)
	ServeSessions(base, sm)
	ServeGroups(base, d)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", conf.Server.Port),
		Handler: r,
	}

	// Initializing the server in a goroutine so that
	// it won't block the graceful shutdown handling below
	go func() {
		log.Info("server listening ", srv.Addr)
		logRoutes(r.Routes())
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("listen: %s\n", err)
		}
	}()

	<-ctx.Done()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error("server forced to shutdown: ", err)
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

func ServeSessions(group *gin.RouterGroup, sm *session.Manager) {
	group.GET(sessionsListPath, GetSessions(sm))
}

func ServeGroups(group *gin.RouterGroup, d *provider.Discovery) {
	group.GET(groupsListPath, GetGroups(d))
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

func ServeThemes(group *gin.RouterGroup, t *theme.Themes) {
	group.GET(listThemesPath, GetThemes(t))
	group.GET(previewThemePath, PreviewTheme(t))
}

func logRoutes(routes gin.RoutesInfo) {
	for _, route := range routes {
		log.Info(fmt.Sprintf("%s %s %s", route.Method, route.Path, route.Handler))
	}
}
