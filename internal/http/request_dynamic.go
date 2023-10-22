package http

import (
	"net/http"

	"github.com/acouvreur/sablier/app/http/pages"
	"github.com/acouvreur/sablier/app/http/routes/models"
	"github.com/acouvreur/sablier/version"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

func RequestDynamic(c *gin.Context) {
	request := models.DynamicRequest{
		Theme:            s.StrategyConfig.Dynamic.DefaultTheme,
		ShowDetails:      s.StrategyConfig.Dynamic.ShowDetailsByDefault,
		RefreshFrequency: s.StrategyConfig.Dynamic.DefaultRefreshFrequency,
		SessionDuration:  s.SessionsConfig.DefaultDuration,
	}

	if err := c.ShouldBind(&request); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	var sessionState *sessions.SessionState
	if len(request.Names) > 0 {
		sessionState = s.SessionsManager.RequestSession(request.Names, request.SessionDuration)
	} else {
		sessionState = s.SessionsManager.RequestSessionGroup(request.Group, request.SessionDuration)
	}

	if sessionState == nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

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
		RefreshFrequency:    request.RefreshFrequency,
		InstanceStates:      sessionStateToRenderOptionsInstanceState(sessionState),
	}

	c.Header("Content-Type", "text/html")
	if err := pages.Render(renderOptions, c.Writer); err != nil {
		log.Error(err)
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
}
