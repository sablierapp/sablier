package api

import (
	"net/http"
	"time"

	"github.com/acouvreur/sablier/internal/session"
	"github.com/acouvreur/sablier/version"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

type DynamicRequestByNames struct {
	Names            []string      `json:"names,omitempty"`
	ShowDetails      bool          `json:"show_details,omitempty"`
	DisplayName      string        `json:"display_name,omitempty"`
	Theme            string        `json:"theme,omitempty"`
	SessionDuration  time.Duration `json:"session_duration,omitempty"`
	RefreshFrequency time.Duration `json:"refresh_frequency,omitempty"`
}

type DynamicRequestByGroup struct {
	Group            string        `form:"group"`
	ShowDetails      bool          `form:"show_details"`
	DisplayName      string        `form:"display_name"`
	Theme            string        `form:"theme"`
	SessionDuration  time.Duration `form:"session_duration"`
	RefreshFrequency time.Duration `form:"refresh_frequency"`
}

func (s *SessionHandler) RequestDynamicByNames(c *gin.Context) {

	var body DynamicRequestByNames

	if err := c.ShouldBindJSON(&body); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	/* request := models.DynamicRequest{
		Theme:            s.StrategyConfig.Dynamic.DefaultTheme,
		ShowDetails:      s.StrategyConfig.Dynamic.ShowDetailsByDefault,
		RefreshFrequency: s.StrategyConfig.Dynamic.DefaultRefreshFrequency,
		SessionDuration:  s.SessionsConfig.DefaultDuration,
	} */

	var ses session.SessionManager

	ses.Request(c)
	var session *sessions.SessionState
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
