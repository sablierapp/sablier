package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/acouvreur/sablier/internal/session"
	"github.com/gin-gonic/gin"
)

type BlockingRequestByNames struct {
	Names           []string      `json:"names,omitempty"`
	SessionDuration time.Duration `json:"session_duration"`
	Timeout         time.Duration `json:"timeout"`
}

type BlockingRequestByGroup struct {
	Group           string        `json:"group,omitempty"`
	SessionDuration time.Duration `json:"session_duration"`
	Timeout         time.Duration `json:"timeout"`
}

type RequestBlockingSession struct {
	session session.SessionManager
}

func (rbs *RequestBlockingSession) RequestBlockingByNames(c *gin.Context) {
	var body BlockingRequestByNames
	if err := c.ShouldBindJSON(&body); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	instances, err := rbs.session.RequestBlockingAll(c.Request.Context(), body.Names, session.RequestBlockingOptions{})
	if errors.Is(err, context.DeadlineExceeded) {
		NotReady(c)
		c.AbortWithError(http.StatusGatewayTimeout, err)
		return
	}
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	if session.Ready(instances) {
		Ready(c)
	} else {
		NotReady(c)
	}

	c.JSON(http.StatusOK, map[string]interface{}{"session": instances})
}

func (rbs *RequestBlockingSession) RequestBlockingByGroup(c *gin.Context) {
	var body BlockingRequestByGroup
	if err := c.ShouldBindJSON(&body); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	names := []string{}

	instances, err := rbs.session.RequestBlockingAll(c.Request.Context(), names, session.RequestBlockingOptions{})
	if errors.Is(err, context.DeadlineExceeded) {
		NotReady(c)
		c.AbortWithError(http.StatusGatewayTimeout, err)
		return
	}
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	if session.Ready(instances) {
		Ready(c)
	} else {
		NotReady(c)
	}

	c.JSON(http.StatusOK, map[string]interface{}{"session": instances})
}
