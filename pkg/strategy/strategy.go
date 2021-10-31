package strategy

import (
	"errors"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

// Net client is a custom client to timeout after 2 seconds if the service is not ready
var netClient = &http.Client{
	Timeout: time.Second * 2,
}

type Strategy interface {
	ServeHTTP(rw http.ResponseWriter, req *http.Request)
}

func getServiceStatus(request string) (string, error) {

	// This request wakes up the service if he's scaled to 0
	resp, err := netClient.Get(request)
	if err != nil {
		return "error", err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "parsing error", err
	}

	if resp.StatusCode >= 400 {
		return "error from ondemand service", errors.New(string(body))
	}

	return strings.TrimSuffix(string(body), "\n"), nil
}
