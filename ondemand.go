package traefik_ondemand_plugin

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/acouvreur/traefik-ondemand-plugin/pkg/pages"
)

const defaultDuration = time.Hour

// Net client is a custom client to timeout after 2 seconds if the service is not ready
var netClient = &http.Client{
	Timeout: time.Second * 2,
}

// Config the plugin configuration
type Config struct {
	Name       string
	ServiceUrl string
	Timeout    time.Duration
}

// CreateConfig creates a config with its default values
func CreateConfig() *Config {
	return &Config{
		Timeout: defaultDuration,
	}
}

// Ondemand holds the request for the on demand service
type Ondemand struct {
	request string
	name    string
	next    http.Handler
	config  *Config
}

func buildRequest(url string, name string, timeout time.Duration) (string, error) {
	request := fmt.Sprintf("%s?name=%s&timeout=%s", url, name, timeout.String())
	return request, nil
}

// New function creates the configuration
func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error) {
	if len(config.ServiceUrl) == 0 {
		return nil, fmt.Errorf("serviceUrl cannot be null")
	}

	if len(config.Name) == 0 {
		return nil, fmt.Errorf("name cannot be null")
	}

	request, err := buildRequest(config.ServiceUrl, config.Name, config.Timeout)

	if err != nil {
		return nil, fmt.Errorf("error while building request")
	}

	return &Ondemand{
		next:    next,
		name:    name,
		request: request,
		config:  config,
	}, nil
}

// ServeHTTP retrieve the service status
func (e *Ondemand) ServeHTTP(rw http.ResponseWriter, req *http.Request) {

	status, err := getServiceStatus(e.request)

	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte(pages.GetErrorPage(e.name, err.Error())))
	}

	if status == "started" {
		// Service started forward request
		e.next.ServeHTTP(rw, req)

	} else if status == "starting" {
		// Service starting, notify client
		rw.WriteHeader(http.StatusAccepted)
		rw.Write([]byte(pages.GetLoadingPage(e.name, e.config.Timeout)))
	} else {
		// Error
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte(pages.GetErrorPage(e.name, status)))
	}
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

	return strings.TrimSuffix(string(body), "\n"), nil
}
