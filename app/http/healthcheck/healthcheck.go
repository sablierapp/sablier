package healthcheck

import (
	"context"
	"io"
	"net/http"
	"time"
)

const (
	healthy   = true
	unhealthy = false
)

// Health performs a basic HTTP health check
// #nosec G107 -- This is intended to be used with variable URLs for health checks
func Health(url string) (string, bool) {
	resp, err := http.Get(url)

	if err != nil {
		return err.Error(), unhealthy
	}

	body, err := io.ReadAll(resp.Body)
	defer resp.Body.Close()

	if err != nil {
		return err.Error(), unhealthy
	}

	if resp.StatusCode >= 400 {
		return string(body), unhealthy
	}

	return string(body), healthy
}

// DockerHealth checks if Docker API is responsive
func DockerHealth(dockerPingFunc func(context.Context) error) (string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := dockerPingFunc(ctx)
	if err != nil {
		return "Docker API connection issue: " + err.Error(), unhealthy
	}

	return "Docker API connection healthy", healthy
}
