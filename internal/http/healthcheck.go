package http

import (
	"io"
	"net/http"
)

const (
	healthy   = true
	unhealthy = false
)

func Health(url string) (string, bool) {
	resp, err := http.Get(url)

	if err != nil {
		return err.Error(), unhealthy
	}

	body, err := io.ReadAll(resp.Body)

	if err != nil {
		return err.Error(), unhealthy
	}

	if resp.StatusCode >= 400 {
		return string(body), unhealthy
	}

	return string(body), healthy
}
