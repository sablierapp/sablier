package api

import (
	"context"
	"errors"
	"github.com/acouvreur/sablier/internal/provider"
	"github.com/acouvreur/sablier/internal/session"
	"github.com/acouvreur/sablier/pkg/durations"
	"github.com/gin-gonic/gin"
	log "log/slog"
	"net/http"
)

type BlockingSessionRequestDefaults struct {
	SessionDuration durations.Duration
	Timeout         durations.Duration
	DesiredReplicas uint32
}

type BlockingSessionRequestByNames struct {
	Names           []string           `json:"names,omitempty"`
	SessionDuration durations.Duration `json:"sessionDuration,format:units"`
	Timeout         durations.Duration `json:"timeout,format:units"`
	DesiredReplicas uint32             `json:"desiredReplicas"`
}

type BlockingSessionRequestByGroup struct {
	Group           string             `json:"group,omitempty"`
	SessionDuration durations.Duration `json:"sessionDuration,format:units"`
	Timeout         durations.Duration `json:"timeout,format:units"`
	DesiredReplicas uint32             `json:"desiredReplicas"`
}

type BlockingSessionResponse struct {
	Instances []session.Instance `json:"instances,omitempty"`
}

type RequestBlockingSession struct {
	defaults  BlockingSessionRequestDefaults
	session   *session.Manager
	discovery *provider.Discovery
}

func (rbs *RequestBlockingSession) RequestBlockingByNames(c *gin.Context) {
	body := BlockingSessionRequestByNames{
		SessionDuration: rbs.defaults.SessionDuration,
		Timeout:         rbs.defaults.Timeout,
		DesiredReplicas: rbs.defaults.DesiredReplicas,
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		log.Warn(err.Error())
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	if len(body.Names) == 0 {
		c.AbortWithStatus(http.StatusBadRequest)
	}

	rbs.requestBlocking(c, body)
}

func (rbs *RequestBlockingSession) RequestBlockingByGroup(c *gin.Context) {
	body := BlockingSessionRequestByGroup{
		SessionDuration: rbs.defaults.SessionDuration,
		Timeout:         rbs.defaults.Timeout,
		DesiredReplicas: rbs.defaults.DesiredReplicas,
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		log.Warn(err.Error())
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	names, found := rbs.discovery.Group(body.Group)
	if !found || len(names) == 0 {
		c.AbortWithStatus(http.StatusNotFound)
	}

	req := BlockingSessionRequestByNames{
		Names:           names,
		SessionDuration: body.SessionDuration,
		Timeout:         body.Timeout,
		DesiredReplicas: body.DesiredReplicas,
	}

	rbs.requestBlocking(c, req)
}

func (rbs *RequestBlockingSession) requestBlocking(c *gin.Context, req BlockingSessionRequestByNames) {
	ctx, cancel := context.WithTimeout(c, req.Timeout.Duration)
	defer cancel()

	instances, err := rbs.session.RequestBlockingAll(ctx, req.Names, session.RequestBlockingOptions{
		DesiredReplicas: req.DesiredReplicas,
		SessionDuration: req.SessionDuration.Duration,
	})
	if errors.Is(err, context.DeadlineExceeded) {
		NotReady(c)
		log.Error("error:", err)
		c.AbortWithError(http.StatusGatewayTimeout, err)
		return
	}
	if err != nil {
		log.Error("error:", err)
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	applyStatusHeader(c, instances)

	c.JSON(http.StatusOK, BlockingSessionResponse{Instances: instances})
}
