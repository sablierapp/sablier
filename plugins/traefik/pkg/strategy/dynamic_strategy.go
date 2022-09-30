package strategy

import (
	"log"
	"net/http"
	"time"

	"github.com/acouvreur/sablier/plugins/traefik/pkg/pages"
)

type DynamicStrategy struct {
	Requests    []string
	Name        string
	Next        http.Handler
	Timeout     time.Duration
	DisplayName string
	LoadingPage string
	ErrorPage   string
}

// ServeHTTP retrieve the service status
func (e *DynamicStrategy) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	started := make([]bool, len(e.Requests))

	displayName := e.Name
	if len(e.DisplayName) > 0 {
		displayName = e.DisplayName
	}

	notReadyCount := 0
	for requestIndex, request := range e.Requests {
		log.Printf("Sending request: %s", request)
		status, err := getServiceStatus(request)
		log.Printf("Status: %s", status)

		if err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			rw.Write([]byte(pages.GetErrorPage(e.ErrorPage, displayName, err.Error())))
			return
		}

		if status == "started" {
			started[requestIndex] = true
		} else if status == "starting" {
			started[requestIndex] = false
			notReadyCount++
		} else {
			// Error
			rw.WriteHeader(http.StatusInternalServerError)
			rw.Write([]byte(pages.GetErrorPage(e.ErrorPage, displayName, status)))
			return
		}
	}
	if notReadyCount == 0 {
		// All services are ready, forward request
		e.Next.ServeHTTP(rw, req)
	} else {
		// Services still starting, notify client
		rw.WriteHeader(http.StatusAccepted)
		rw.Write([]byte(pages.GetLoadingPage(e.LoadingPage, displayName, e.Timeout)))
	}
}
