//go:build e2e
// +build e2e

package e2e

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gavv/httpexpect/v2"
)

func Test_Dynamic(t *testing.T) {
	e := httpexpect.Default(t, "http://localhost:8080/dynamic/")

	e.GET("/whoami").
		Expect().
		Status(http.StatusOK).
		Body().
		Contains(`Dynamic Whoami`).
		Contains(`Your instance(s) will stop after 1 minute of inactivity`)

	e.GET("/whoami").
		WithMaxRetries(20).
		WithRetryDelay(50*time.Millisecond, time.Second*2).
		WithRetryPolicy(httpexpect.RetryCustomHandler).
		WithCustomHandler(RetryUntilBodyContains("Host: localhost:8080")).
		Expect().
		Status(http.StatusOK).
		Body().Contains(`Host: localhost:8080`)
}

func Test_Blocking(t *testing.T) {
	e := httpexpect.Default(t, "http://localhost:8080/blocking/")

	e.GET("/whoami").
		Expect().
		Status(http.StatusOK).
		Body().Contains(`Host: localhost:8080`)
}

func Test_Multiple(t *testing.T) {
	e := httpexpect.Default(t, "http://localhost:8080/multiple/")

	e.GET("/whoami").
		Expect().
		Status(http.StatusOK).
		Body().
		Contains(`Multiple Whoami`).
		Contains(`Your instance(s) will stop after 1 minute of inactivity`)

	e.GET("/whoami").
		WithMaxRetries(20).
		WithRetryDelay(50*time.Millisecond, time.Second*2).
		WithRetryPolicy(httpexpect.RetryCustomHandler).
		WithCustomHandler(RetryUntilBodyContains("Host: localhost:8080")).
		Expect().
		Status(http.StatusOK).
		Body().Contains(`Host: localhost:8080`)

	e.GET("/nginx").
		WithMaxRetries(20).
		WithRetryDelay(50*time.Millisecond, time.Second*2).
		WithRetryPolicy(httpexpect.RetryCustomHandler).
		WithCustomHandler(RetryUntilBodyContains("nginx/")).
		Expect().
		Status(http.StatusNotFound).
		Body().Contains(`nginx/`)
}

func Test_Healthy(t *testing.T) {
	e := httpexpect.Default(t, "http://localhost:8080/healthy/")

	e.GET("/nginx").
		Expect().
		Status(http.StatusOK).
		Body().
		Contains(`Healthy Nginx`).
		Contains(`Your instance(s) will stop after 1 minute of inactivity`)

	e.GET("/nginx").
		WithMaxRetries(20).
		WithRetryDelay(50*time.Millisecond, time.Second*2).
		WithRetryPolicy(httpexpect.RetryCustomHandler).
		WithCustomHandler(RetryUntilBodyContains("nginx/")).
		Expect().
		Status(http.StatusNotFound).
		Body().Contains(`nginx/`)
}

func Test_Group(t *testing.T) {
	e := httpexpect.Default(t, "http://localhost:8080/")

	e.GET("/group").
		Expect().
		Status(http.StatusOK).
		Body().
		Contains(`Group E2E`).
		Contains(`Your instance(s) will stop after 1 minute of inactivity`)

	e.GET("/group").
		WithMaxRetries(20).
		WithRetryDelay(50*time.Millisecond, time.Second*2).
		WithRetryPolicy(httpexpect.RetryCustomHandler).
		WithCustomHandler(RetryUntilBodyContains("Host: localhost:8080")).
		Expect().
		Status(http.StatusOK).
		Body().Contains(`Host: localhost:8080`)
}

func RetryUntilBodyContains(contains string) func(resp *http.Response, err error) bool {
	return func(resp *http.Response, err error) bool {
		if err != nil {
			return true
		}
		if resp.Body != nil {
			// Check body if available, etc.
			body, err := io.ReadAll(resp.Body)
			defer resp.Body.Close()
			if err != nil {
				return true
			}
			return !strings.Contains(string(body), contains)
		}
		return true
	}
}
