//go:build e2e
// +build e2e

package e2e

import (
	"net/http"
	"testing"
	"time"

	"github.com/gavv/httpexpect/v2"
)

var waitingTime = 10 * time.Second

func Test_Dynamic(t *testing.T) {
	e := httpexpect.New(t, "http://localhost:8080/dynamic/")

	e.GET("/whoami").
		Expect().
		Status(http.StatusOK).
		Body().
		Contains(`Dynamic Whoami`).
		Contains(`Your instance(s) will stop after 1 minutes of inactivity`)

	time.Sleep(waitingTime)

	e.GET("/whoami").
		Expect().
		Status(http.StatusOK).
		Body().Contains(`Host: localhost:8080`)
}

func Test_Blocking(t *testing.T) {
	e := httpexpect.New(t, "http://localhost:8080/blocking/")

	e.GET("/whoami").
		Expect().
		Status(http.StatusOK).
		Body().Contains(`Host: localhost:8080`)
}

func Test_Multiple(t *testing.T) {
	e := httpexpect.New(t, "http://localhost:8080/multiple/")

	e.GET("/whoami").
		Expect().
		Status(http.StatusOK).
		Body().
		Contains(`Multiple Whoami`).
		Contains(`Your instance(s) will stop after 1 minutes of inactivity`)

	time.Sleep(waitingTime)

	e.GET("/whoami").
		Expect().
		Status(http.StatusOK).
		Body().Contains(`Host: localhost:8080`)

	e.GET("/nginx").
		Expect().
		Status(http.StatusNotFound).
		Body().Contains(`nginx/1.23.1`)
}

func Test_Healthy(t *testing.T) {
	e := httpexpect.New(t, "http://localhost:8080/healthy/")

	e.GET("/nginx").
		Expect().
		Status(http.StatusOK).
		Body().
		Contains(`Healthy Nginx`).
		Contains(`Your instance(s) will stop after 1 minutes of inactivity`)

	e.GET("/nginx").
		Expect().
		Status(http.StatusOK).
		Body().
		Contains(`Healthy Nginx`).
		Contains(`Your instance(s) will stop after 1 minutes of inactivity`)

	time.Sleep(waitingTime)

	e.GET("/nginx").
		Expect().
		Status(http.StatusNotFound).
		Body().Contains(`nginx/1.23.1`)
}
