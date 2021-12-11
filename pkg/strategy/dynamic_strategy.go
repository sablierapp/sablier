package strategy

import (
	"log"
	"net/http"
	"time"

	"github.com/acouvreur/traefik-ondemand-plugin/pkg/pages"
)

type DynamicStrategy struct {
	Request     string
	Name        string
	Next        http.Handler
	Timeout     time.Duration
	LoadingPage string
	ErrorPage   string
}

// ServeHTTP retrieve the service status
func (e *DynamicStrategy) ServeHTTP(rw http.ResponseWriter, req *http.Request) {

	log.Printf("Sending request: %s", e.Request)
	status, err := getServiceStatus(e.Request)
	log.Printf("Status: %s", status)

	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte(pages.GetErrorPage(e.ErrorPage, e.Name, err.Error())))
	}

	if status == "started" {
		// Service started forward request
		e.Next.ServeHTTP(rw, req)

	} else if status == "starting" {
		// Service starting, notify client
		rw.WriteHeader(http.StatusAccepted)
		rw.Write([]byte(pages.GetLoadingPage(e.LoadingPage, e.Name, e.Timeout)))
	} else {
		// Error
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte(pages.GetErrorPage(e.ErrorPage, e.Name, status)))
	}

}
