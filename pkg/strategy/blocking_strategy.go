package strategy

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

type BlockingStrategy struct {
	Request            string
	Name               string
	Next               http.Handler
	Timeout            time.Duration
	BlockDelay         time.Duration
	BlockCheckInterval time.Duration
}

type InternalServerError struct {
	ServiceName string `json:"serviceName"`
	Error       string `json:"error"`
}

// ServeHTTP retrieve the service status
func (e *BlockingStrategy) ServeHTTP(rw http.ResponseWriter, req *http.Request) {

	for start := time.Now(); time.Since(start) < e.BlockDelay; {
		log.Printf("Sending request: %s", e.Request)
		status, err := getServiceStatus(e.Request)
		log.Printf("Status: %s", status)

		if err != nil {
			rw.Header().Set("Content-Type", "application/json")
			rw.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(rw).Encode(InternalServerError{ServiceName: e.Name, Error: err.Error()})
			return
		}

		if status == "started" {
			// Service started forward request
			e.Next.ServeHTTP(rw, req)
			return
		}
		time.Sleep(e.BlockCheckInterval)
	}

	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(http.StatusServiceUnavailable)
	json.NewEncoder(rw).Encode(InternalServerError{ServiceName: e.Name, Error: fmt.Sprintf("Service was unreachable within %s", e.BlockDelay)})
}
