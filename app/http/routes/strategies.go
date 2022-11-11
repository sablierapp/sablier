package routes

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/acouvreur/sablier/app/http/pages"
	"github.com/acouvreur/sablier/app/http/routes/models"
	"github.com/acouvreur/sablier/app/instance"
	"github.com/acouvreur/sablier/app/sessions"
	"github.com/acouvreur/sablier/config"
	"github.com/acouvreur/sablier/version"
	"github.com/gin-gonic/gin"
)

var osDirFS = os.DirFS

type ServeStrategy struct {
	customThemesFS fs.FS
	customThemes   map[string]bool

	SessionsManager sessions.Manager
	StrategyConfig  config.Strategy
}

func NewServeStrategy(sessionsManager sessions.Manager, conf config.Strategy) *ServeStrategy {

	serveStrategy := &ServeStrategy{
		SessionsManager: sessionsManager,
		StrategyConfig:  conf,
	}

	if conf.Dynamic.CustomThemesPath != "" {
		customThemesFs := osDirFS(conf.Dynamic.CustomThemesPath)
		serveStrategy.customThemesFS = customThemesFs
		serveStrategy.customThemes = listThemes(customThemesFs)
	}

	return serveStrategy
}

func (s *ServeStrategy) ServeDynamic(c *gin.Context) {
	request := models.DynamicRequest{
		Theme:            s.StrategyConfig.Dynamic.DefaultTheme,
		ShowDetails:      s.StrategyConfig.Dynamic.ShowDetailsByDefault,
		RefreshFrequency: s.StrategyConfig.Dynamic.DefaultRefreshFrequency,
	}

	if err := c.ShouldBind(&request); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	sessionState := s.SessionsManager.RequestSession(request.Names, request.SessionDuration)

	if sessionState.IsReady() {
		c.Header("X-Sablier-Session-Status", "ready")
	} else {
		c.Header("X-Sablier-Session-Status", "not-ready")
	}

	renderOptions := pages.RenderOptions{
		DisplayName:         request.DisplayName,
		ShowDetails:         request.ShowDetails,
		SessionDuration:     request.SessionDuration,
		Theme:               request.Theme,
		CustomThemes:        s.customThemesFS,
		AllowedCustomThemes: s.customThemes,
		Version:             version.Version,
		RefreshFrequency:    5 * time.Second,
		InstanceStates:      sessionStateToRenderOptionsInstanceState(sessionState),
	}

	c.Header("Content-Type", "text/html")
	if err := pages.Render(renderOptions, c.Writer); err != nil {
		log.Error(err)
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
}

func (s *ServeStrategy) ServeDynamicThemes(c *gin.Context) {

	customThemes := []string{}
	for theme := range s.customThemes {
		customThemes = append(customThemes, theme)
	}
	sort.Strings(customThemes)

	embeddedThemes := []string{}
	for theme := range listThemes(pages.Themes) {
		embeddedThemes = append(embeddedThemes, strings.TrimPrefix(theme, "themes/"))
	}
	sort.Strings(embeddedThemes)

	c.JSON(http.StatusOK, map[string]interface{}{
		"custom":   customThemes,
		"embedded": embeddedThemes,
	})
}

func (s *ServeStrategy) ServeBlocking(c *gin.Context) {
	request := models.BlockingRequest{
		Timeout: s.StrategyConfig.Blocking.DefaultTimeout,
	}

	if err := c.ShouldBind(&request); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	sessionState, err := s.SessionsManager.RequestReadySession(c.Request.Context(), request.Names, request.SessionDuration, request.Timeout)

	if err != nil {
		c.Header("X-Sablier-Session-Status", "not-ready")
		c.JSON(http.StatusGatewayTimeout, map[string]interface{}{"error": err.Error()})
		return
	}

	if sessionState.IsReady() {
		c.Header("X-Sablier-Session-Status", "ready")
	} else {
		c.Header("X-Sablier-Session-Status", "not-ready")
	}

	c.JSON(http.StatusOK, map[string]interface{}{"session": sessionState})
}

func sessionStateToRenderOptionsInstanceState(sessionState *sessions.SessionState) (instances []pages.RenderOptionsInstanceState) {
	sessionState.Instances.Range(func(key, value any) bool {
		instances = append(instances, instanceStateToRenderOptionsRequestState(value.(sessions.InstanceState).Instance))
		return true
	})

	sort.SliceStable(instances, func(i, j int) bool {
		return strings.Compare(instances[i].Name, instances[j].Name) == -1
	})

	return
}

func instanceStateToRenderOptionsRequestState(instanceState *instance.State) pages.RenderOptionsInstanceState {

	var err error
	if instanceState.Message == "" {
		err = nil
	} else {
		err = fmt.Errorf(instanceState.Message)
	}

	return pages.RenderOptionsInstanceState{
		Name:            instanceState.Name,
		Status:          instanceState.Status,
		CurrentReplicas: instanceState.CurrentReplicas,
		DesiredReplicas: instanceState.DesiredReplicas,
		Error:           err,
	}
}

func listThemes(dir fs.FS) (themes map[string]bool) {
	themes = make(map[string]bool)
	fs.WalkDir(dir, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		if strings.HasSuffix(d.Name(), ".html") {
			log.Debugf("found theme at \"%s\" can be loaded using \"%s\"", path, strings.TrimSuffix(path, ".html"))
			themes[strings.TrimSuffix(path, ".html")] = true
		} else {
			log.Tracef("ignoring file \"%s\" because it has no .html suffix", path)
		}
		return nil
	})
	return
}
