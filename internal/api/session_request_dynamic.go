package api

import (
	"github.com/acouvreur/sablier/internal/provider"
	"github.com/acouvreur/sablier/internal/session"
	"github.com/acouvreur/sablier/internal/theme"
	"github.com/acouvreur/sablier/pkg/durations"
	"github.com/gin-gonic/gin"
	log "log/slog"
	"net/http"
	"sort"
)

type DynamicRequestThemeOptions struct {
	Title            string             `json:"title"`
	DisplayName      string             `json:"displayName"`
	ShowDetails      bool               `json:"showDetails"`
	RefreshFrequency durations.Duration `json:"refreshFrequency,format:units"`
}

type DynamicSessionRequestDefaults struct {
	SessionDuration durations.Duration
	Theme           string
	ThemeOptions    DynamicRequestThemeOptions
	DesiredReplicas uint32
}

type DynamicSessionRequestByNames struct {
	Names           []string                   `json:"names,omitempty"`
	SessionDuration durations.Duration         `json:"sessionDuration,format:units"`
	Theme           string                     `json:"theme"`
	ThemeOptions    DynamicRequestThemeOptions `json:"themeOptions"`
	DesiredReplicas uint32                     `json:"desiredReplicas"`
}

type DynamicSessionRequestByGroup struct {
	Group           string                     `json:"group,omitempty"`
	SessionDuration durations.Duration         `json:"sessionDuration,format:units"`
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
	body := DynamicSessionRequestByNames{
		SessionDuration: rds.defaults.SessionDuration,
		Theme:           rds.defaults.Theme,
		ThemeOptions:    rds.defaults.ThemeOptions,
		DesiredReplicas: rds.defaults.DesiredReplicas,
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	if len(body.Names) == 0 {
		c.AbortWithStatus(http.StatusBadRequest)
	}

	rds.requestDynamic(c, body)
}

func (rds *RequestDynamicSession) RequestDynamicByGroup(c *gin.Context) {
	body := DynamicSessionRequestByGroup{
		SessionDuration: rds.defaults.SessionDuration,
		Theme:           rds.defaults.Theme,
		ThemeOptions:    rds.defaults.ThemeOptions,
		DesiredReplicas: rds.defaults.DesiredReplicas,
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		log.Error("error:", err)
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	names, found := rds.discovery.Group(body.Group)
	if !found || len(names) == 0 {
		c.AbortWithStatus(http.StatusNotFound)
	}

	req := DynamicSessionRequestByNames{
		Names:           names,
		SessionDuration: body.SessionDuration,
		Theme:           body.Theme,
		ThemeOptions:    body.ThemeOptions,
		DesiredReplicas: body.DesiredReplicas,
	}

	rds.requestDynamic(c, req)
}

func (rds *RequestDynamicSession) requestDynamic(c *gin.Context, req DynamicSessionRequestByNames) {
	instances, err := rds.session.RequestDynamic(c.Request.Context(), req.Names, session.RequestDynamicOptions{
		DesiredReplicas: req.DesiredReplicas,
		SessionDuration: req.SessionDuration.Duration,
	})
	if err != nil {
		log.Error("error:", err)
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	instancesInfo := make([]theme.InstanceInfo, len(instances))
	for i, instance := range instances {
		instancesInfo[i] = theme.InstanceInfo{
			Name:   instance.Name,
			Status: string(instance.Status),
			Error:  instance.Error,
		}
	}

	sort.SliceStable(instancesInfo, func(i, j int) bool {
		return instancesInfo[i].Name < instancesInfo[j].Name
	})

	opts := theme.Options{
		Title:            req.ThemeOptions.Title,
		DisplayName:      req.ThemeOptions.DisplayName,
		ShowDetails:      req.ThemeOptions.ShowDetails,
		Instances:        instancesInfo,
		SessionDuration:  req.SessionDuration.Duration,
		RefreshFrequency: req.ThemeOptions.RefreshFrequency.Duration,
	}

	applyStatusHeader(c, instances)

	c.Header("Content-Type", "text/html")
	if err := rds.theme.Execute(c.Writer, req.Theme, opts); err != nil {
		log.Error("error:", err)
		c.AbortWithError(http.StatusInternalServerError, err)
	}
}
