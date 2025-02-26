package api

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type Health struct {
	TerminatingStatusCode int `description:"Terminating status code" json:"terminatingStatusCode,omitempty" yaml:"terminatingStatusCode,omitempty" export:"true"`
	terminating           bool
	dockerPingFunc        func(context.Context) error
	dockerPingStatus      bool
	dockerPingMessage     string
	dockerPingMutex       sync.RWMutex
	dockerPingLastCheck   time.Time
}

func (h *Health) SetDefaults() {
	h.TerminatingStatusCode = http.StatusServiceUnavailable
	h.dockerPingStatus = true // Assume healthy at start
	h.dockerPingMessage = "Docker health check not yet performed"
}

func (h *Health) WithContext(ctx context.Context) {
	go func() {
		<-ctx.Done()
		h.terminating = true
	}()
}

func (h *Health) WithDockerPing(pingFunc func(context.Context) error) {
	h.dockerPingFunc = pingFunc
	go h.startDockerHealthCheck(context.Background())
}

func (h *Health) startDockerHealthCheck(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.checkDockerHealth(ctx)
		}
	}
}

func (h *Health) checkDockerHealth(ctx context.Context) {
	if h.dockerPingFunc == nil {
		return
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	err := h.dockerPingFunc(pingCtx)

	h.dockerPingMutex.Lock()
	defer h.dockerPingMutex.Unlock()

	h.dockerPingLastCheck = time.Now()
	if err != nil {
		h.dockerPingStatus = false
		h.dockerPingMessage = "Docker API connection issue: " + err.Error()
	} else {
		h.dockerPingStatus = true
		h.dockerPingMessage = "Docker API connection healthy"
	}
}

func (h *Health) ServeHTTP(c *gin.Context) {
	statusCode := http.StatusOK
	if h.terminating {
		statusCode = h.TerminatingStatusCode
		c.String(statusCode, http.StatusText(statusCode))
		return
	}

	// Check Docker health status if we're using Docker provider
	h.dockerPingMutex.RLock()
	dockerHealthy := h.dockerPingStatus
	dockerMessage := h.dockerPingMessage
	lastCheck := h.dockerPingLastCheck
	h.dockerPingMutex.RUnlock()

	// If Docker is not healthy and Docker provider is being used, return unhealthy
	if h.dockerPingFunc != nil && !dockerHealthy {
		statusCode = http.StatusServiceUnavailable
		c.JSON(statusCode, gin.H{
			"status":        "unhealthy",
			"reason":        "Docker connectivity issue",
			"details":       dockerMessage,
			"last_checked":  lastCheck,
		})
		return
	}

	c.JSON(statusCode, gin.H{
		"status":        "healthy",
		"docker_health": dockerMessage,
		"last_checked":  lastCheck,
	})
}

func Healthcheck(router *gin.RouterGroup, ctx context.Context, dockerPingFunc func(context.Context) error) {
	health := Health{}
	health.SetDefaults()
	health.WithContext(ctx)

	if dockerPingFunc != nil {
		health.WithDockerPing(dockerPingFunc)
	}

	router.GET("/health", health.ServeHTTP)
}
