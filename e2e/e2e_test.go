//go:build e2e
// +build e2e

package e2e

import (
	"net/http"
	"testing"
	"time"

	"github.com/gavv/httpexpect/v2"
)

func Test_Dynamic(t *testing.T) {
	e := httpexpect.New(t, "http://localhost:8080/dynamic/")

	e.GET("/whoami").
		Expect().
		Status(http.StatusAccepted).
		Body().
		Contains(`<h2 class="headline" id="headline">Dynamic Whoami is loading...</h2>`).
		Contains(`Your instance will shutdown automatically after 10 seconds of inactivity.`)

	time.Sleep(5 * time.Second)

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
		Status(http.StatusAccepted).
		Body().
		Contains(`<h2 class="headline" id="headline">Multiple Whoami is loading...</h2>`).
		Contains(`Your instance will shutdown automatically after 10 seconds of inactivity.`)

	time.Sleep(5 * time.Second)

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
		Status(http.StatusAccepted).
		Body().
		Contains(`<h2 class="headline" id="headline">Healthy Nginx is loading...</h2>`).
		Contains(`Your instance will shutdown automatically after 20 seconds of inactivity.`)

	e.GET("/nginx").
		Expect().
		Status(http.StatusAccepted).
		Body().
		Contains(`<h2 class="headline" id="headline">Healthy Nginx is loading...</h2>`).
		Contains(`Your instance will shutdown automatically after 20 seconds of inactivity.`)

	time.Sleep(5 * time.Second)

	e.GET("/nginx").
		Expect().
		Status(http.StatusNotFound).
		Body().Contains(`nginx/1.23.1`)
}
