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
	DesiredReplicas uint32
}

type DynamicRequestByNames struct {
	Names           []string                   `json:"names,omitempty"`
	SessionDuration time.Duration              `json:"session_duration"`
	Theme           string                     `json:"theme"`
	ThemeOptions    DynamicRequestThemeOptions `json:"themeOptions"`
	DesiredReplicas uint32                     `json:"desiredReplicas"`
}

type DynamicRequestByGroup struct {
	Group           string                     `json:"group,omitempty"`
	SessionDuration time.Duration              `json:"sessionDuration"`
	Theme           string                     `json:"theme"`
	ThemeOptions    DynamicRequestThemeOptions `json:"themeOptions"`
	DesiredReplicas uint32                     `json:"desiredReplicas"`
}

type RequestDynamicSession struct {
	defaults  DynamicSessionRequestDefaults
	theme     *theme.Themes
	session   *session.Manager
	discovery *provider.Discovery
}

func (rds *RequestDynamicSession) RequestDynamicByNames(c *gin.Context) {
	body := DynamicRequestByNames{
		SessionDuration: rds.defaults.SessionDuration,
		Theme:           rds.defaults.Theme,
		ThemeOptions:    rds.defaults.ThemeOptions,
		DesiredReplicas: rds.defaults.DesiredReplicas,
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	rds.requestDynamic(c, body)
}

func (rds *RequestDynamicSession) RequestDynamicByGroup(c *gin.Context) {
	body := DynamicRequestByGroup{
		SessionDuration: rds.defaults.SessionDuration,
		Theme:           rds.defaults.Theme,
		ThemeOptions:    rds.defaults.ThemeOptions,
		DesiredReplicas: rds.defaults.DesiredReplicas,
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	names, found := rds.discovery.Group(body.Group)
	if !found || len(names) == 0 {
		c.AbortWithStatus(http.StatusNotFound)
	}

	req := DynamicRequestByNames{
		Names:           names,
		SessionDuration: body.SessionDuration,
		Theme:           body.Theme,
		ThemeOptions:    body.ThemeOptions,
		DesiredReplicas: body.DesiredReplicas,
	}

	rds.requestDynamic(c, req)
}

func (rds *RequestDynamicSession) requestDynamic(c *gin.Context, req DynamicRequestByNames) {
	instances, err := rds.session.RequestDynamicAll(c.Request.Context(), req.Names, session.RequestDynamicOptions{
		DesiredReplicas: req.DesiredReplicas,
		SessionDuration: req.SessionDuration,
	})
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	instancesInfo := make([]theme.InstanceInfo, 0, len(instances))
	for _, instance := range instances {
		instancesInfo = append(instancesInfo, theme.InstanceInfo{
			Name:   instance.Name,
			Status: string(instance.Status),
			Error:  instance.Error,
		})
	}

	opts := theme.Options{
		Title:            req.ThemeOptions.Title,
		DisplayName:      req.ThemeOptions.DisplayName,
		ShowDetails:      req.ThemeOptions.ShowDetails,
		Instances:        instancesInfo,
		SessionDuration:  req.SessionDuration,
		RefreshFrequency: req.ThemeOptions.RefreshFrequency,
	}

	applyStatusHeader(c, instances)

	c.Header("Content-Type", "text/html")
	if err := rds.theme.Execute(c.Writer, req.Theme, opts); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
	}
}
