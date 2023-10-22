package routes

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
