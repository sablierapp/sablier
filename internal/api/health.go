package api

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Health struct {
	TerminatingStatusCode int `description:"Terminating status code" json:"terminatingStatusCode,omitempty" yaml:"terminatingStatusCode,omitempty" export:"true"`
	terminating           bool
}

func (h *Health) SetDefaults() {
	h.TerminatingStatusCode = http.StatusServiceUnavailable
}

func (h *Health) WithContext(ctx context.Context) {
	go func() {
		<-ctx.Done()
		h.terminating = true
	}()
}

func (h *Health) ServeHTTP(c *gin.Context) {
	statusCode := http.StatusOK
	if h.terminating {
		statusCode = h.TerminatingStatusCode
	}

	c.String(statusCode, http.StatusText(statusCode))
}

// Healthcheck registers the health endpoint.
//
// @Summary      Health check
// @Description  Liveness endpoint. Returns 200 while serving and 503 while the server is terminating.
// @Tags         system
// @Produce      plain
// @Success      200  {string}  string  "OK"
// @Failure      503  {string}  string  "Service Unavailable"
// @Router       /health [get]
func Healthcheck(router *gin.RouterGroup, ctx context.Context) {
	health := Health{}
	health.SetDefaults()
	health.WithContext(ctx)
	router.GET("/health", health.ServeHTTP)
}
