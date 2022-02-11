package strategy

import (
	"log"
	"net/http"
	"time"

	"github.com/acouvreur/traefik-ondemand-plugin/pkg/pages"
)

type DynamicStrategy struct {
	Requests    []string
	Name        string
	Next        http.Handler
	Timeout     time.Duration
	LoadingPage string
	ErrorPage   string
}

// ServeHTTP retrieve the service status
func (e *DynamicStrategy) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	started := make([]bool, len(e.Requests))
	notReadyCount := 0
	for requestIndex, request := range e.Requests {
		log.Printf("Sending request: %s", request)
		status, err := getServiceStatus(request)
		log.Printf("Status: %s", status)

		if err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			rw.Write([]byte(pages.GetErrorPage(e.ErrorPage, e.Name, err.Error())))
		}

		if status == "started" {
			started[requestIndex] = true
		} else if status == "starting" {
			started[requestIndex] = false
			notReadyCount++
		} else {
			// Error
			rw.WriteHeader(http.StatusInternalServerError)
			rw.Write([]byte(pages.GetErrorPage(e.ErrorPage, e.Name, status)))
		}
	}
	if notReadyCount == 0 {
		// All services are ready, forward request
		e.Next.ServeHTTP(rw, req)
	} else {
		// Services still starting, notify client
		rw.WriteHeader(http.StatusAccepted)
		rw.Write([]byte(pages.GetLoadingPage(e.LoadingPage, e.Name, e.Timeout)))
	}
}
