package api

import (
	"github.com/acouvreur/sablier/internal/provider"
	"github.com/acouvreur/sablier/internal/theme"
	"net/http"
	"time"

	"github.com/acouvreur/sablier/internal/session"
	"github.com/gin-gonic/gin"
)

type DynamicRequestThemeOptions struct {
	Title            string        `json:"title"`
	DisplayName      string        `json:"displayName"`
	ShowDetails      bool          `json:"showDetails"`
	RefreshFrequency time.Duration `json:"refreshFrequency"`
}

type DynamicSessionRequestDefaults struct {
	SessionDuration time.Duration
	Theme           string
	ThemeOptions    DynamicRequestThemeOptions
}

type DynamicRequestByNames struct {
	Names           []string                   `json:"names,omitempty"`
	SessionDuration time.Duration              `json:"session_duration"`
	Theme           string                     `json:"theme"`
	ThemeOptions    DynamicRequestThemeOptions `json:"themeOptions"`
}

type DynamicRequestByGroup struct {
	Group           string                     `json:"group,omitempty"`
	SessionDuration time.Duration              `json:"sessionDuration"`
	Theme           string                     `json:"theme"`
	ThemeOptions    DynamicRequestThemeOptions `json:"themeOptions"`
}

type RequestDynamicSession struct {
	defaults  DynamicSessionRequestDefaults
	theme     theme.Themes
	session   session.SessionManager
	discovery provider.Discovery
}

func (rds *RequestDynamicSession) RequestDynamicByNames(c *gin.Context) error {
	var body DynamicRequestByNames
	if err := c.ShouldBindJSON(&body); err != nil {
		return c.AbortWithError(http.StatusBadRequest, err)
	}

	return rds.requestDynamic(c, body)
}

func (rds *RequestDynamicSession) RequestDynamicByGroup(c *gin.Context) error {
	var body DynamicRequestByGroup
	if err := c.ShouldBindJSON(&body); err != nil {
		return c.AbortWithError(http.StatusBadRequest, err)
	}

	names, found := rds.discovery.Group(body.Group)
	if !found || len(names) == 0 {
		c.AbortWithStatus(http.StatusNotFound)
	}

	req := DynamicRequestByNames{
		Names:           names,
		SessionDuration: body.SessionDuration,
	}

	return rds.requestDynamic(c, req)
}

func (rds *RequestDynamicSession) requestDynamic(c *gin.Context, req DynamicRequestByNames) error {
	instances, err := rds.session.Request(c.Request.Context(), req.Names, session.RequestBlockingOptions{
		DesiredReplicas: req.DesiredReplicas,
		SessionDuration: req.SessionDuration,
	})
	if err != nil {
		return c.AbortWithError(http.StatusInternalServerError, err)
	}

	opts := theme.Options{
		Title:            "",
		DisplayName:      "",
		ShowDetails:      req.ShowDetails,
		Instances:        instances,
		SessionDuration:  req.SessionDuration,
		RefreshFrequency: req.RefreshFrequency,
	}

	applyStatusHeader(c, instances)

	c.Header("Content-Type", "text/html")
	if err := rds.theme.Execute(c.Writer, req.Theme, opts); err != nil {
		return c.AbortWithError(http.StatusInternalServerError, err)
	}
	return nil
}
