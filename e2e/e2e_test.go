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
	"github.com/gorilla/websocket"
)

func Test_Blocking_WebSocket(t *testing.T) {
	wsURL := "ws://localhost:8080/echo"

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal("WebSocket connection failed:", err)
	}
	defer conn.Close()

	done := make(chan bool)
	go func() {
		defer close(done)
		endTime := time.Now().Add(1 * time.Minute)
		for time.Now().Before(endTime) {
			if err := conn.WriteMessage(websocket.TextMessage, []byte("Hello WebSocket")); err != nil {
				t.Error("Write error:", err)
				return
			}
			_, message, err := conn.ReadMessage()
			if err != nil {
				t.Error("Read error:", err)
				return
			}
			t.Logf("Received: %s", message)
			time.Sleep(20 * time.Second)
		}
	}()

	select {
	case <-done:
	case <-time.After(time.Minute * 2 + 30*time.Second):
		t.Fatal("Test did not complete in time")
	}
}

func Test_Dynamic(t *testing.T) {
	e := httpexpect.Default(t, "http://localhost:8080/dynamic/")

	e.GET("/whoami").
		Expect().
		Status(http.StatusOK).
		Body().
		Contains(`Dynamic Whoami`).
		Contains(`Your instance(s) will stop after 1 minute of inactivity`)

	e.GET("/whoami").
		WithMaxRetries(10).
		WithRetryDelay(time.Second, time.Second*2).
		WithRetryPolicy(httpexpect.RetryCustomHandler).
		WithCustomHandler(func(resp *http.Response, _ error) bool {
			if resp.Body != nil {

				// Check body if available, etc.
				body, err := io.ReadAll(resp.Body)
				defer resp.Body.Close()
				if err != nil {
					return true
				}
				return !strings.Contains(string(body), "Host: localhost:8080")
			}
			return false
		}).
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
		WithMaxRetries(10).
		WithRetryDelay(time.Second, time.Second*2).
		WithRetryPolicy(httpexpect.RetryCustomHandler).
		WithCustomHandler(func(resp *http.Response, _ error) bool {
			if resp.Body != nil {
				// Check body if available, etc.
				body, err := io.ReadAll(resp.Body)
				defer resp.Body.Close()
				if err != nil {
					return true
				}
				return !strings.Contains(string(body), "Host: localhost:8080")
			}
			return false
		}).
		Expect().
		Status(http.StatusOK).
		Body().Contains(`Host: localhost:8080`)

	e.GET("/nginx").
		WithMaxRetries(10).
		WithRetryDelay(time.Second, time.Second*2).
		WithRetryPolicy(httpexpect.RetryCustomHandler).
		WithCustomHandler(func(resp *http.Response, _ error) bool {
			if resp.Body != nil {

				// Check body if available, etc.
				body, err := io.ReadAll(resp.Body)
				defer resp.Body.Close()
				if err != nil {
					return true
				}
				return !strings.Contains(string(body), "nginx/")
			}
			return false
		}).
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
		WithMaxRetries(10).
		WithRetryDelay(time.Second, time.Second*2).
		WithRetryPolicy(httpexpect.RetryCustomHandler).
		WithCustomHandler(func(resp *http.Response, _ error) bool {
			if resp.Body != nil {

				// Check body if available, etc.
				body, err := io.ReadAll(resp.Body)
				defer resp.Body.Close()
				if err != nil {
					return true
				}
				return !strings.Contains(string(body), "nginx/")
			}
			return false
		}).
		Expect().
		Status(http.StatusNotFound).
		Body().Contains(`nginx/`)
}
