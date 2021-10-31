package strategy

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/acouvreur/traefik-ondemand-plugin/pkg/pages"
)

type BlockingStrategy struct {
	Request    string
	Name       string
	Next       http.Handler
	Timeout    time.Duration
	BlockDelay time.Duration
}

// ServeHTTP retrieve the service status
func (e *BlockingStrategy) ServeHTTP(rw http.ResponseWriter, req *http.Request) {

	// TODO: Wait for 
	for start := time.Now(); time.Since(start) < e.BlockDelay; {
		log.Printf("Sending request: %s", e.Request)
		status, err := getServiceStatus(e.Request)
		log.Printf("Status: %s", status)

		if err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			rw.Write([]byte(pages.GetErrorPage(e.Name, err.Error())))
			return
		}

		if status == "started" {
			// Service started forward request
			e.Next.ServeHTTP(rw, req)
			return
		}
	}

	rw.WriteHeader(http.StatusServiceUnavailable)
	rw.Write([]byte(pages.GetErrorPage(e.Name, fmt.Sprintf("Service was unreachable within %s", e.BlockDelay))))
}
