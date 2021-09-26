package traefik_ondemand_plugin

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/acouvreur/traefik-ondemand-plugin/pkg/pages"
)

// Net client is a custom client to timeout after 2 seconds if the service is not ready
var netClient = &http.Client{
	Timeout: time.Second * 2,
}

// Config the plugin configuration
type Config struct {
	Name       string `yaml:"name"`
	ServiceUrl string `yaml:"serviceurl"`
	Timeout    string `yaml:"timeout"`
}

// CreateConfig creates a config with its default values
func CreateConfig() *Config {
	return &Config{
		Timeout: "1m",
	}
}

// Ondemand holds the request for the on demand service
type Ondemand struct {
	request string
	name    string
	next    http.Handler
	timeout time.Duration
}

func buildRequest(url string, name string, timeout time.Duration) (string, error) {
	request := fmt.Sprintf("%s?name=%s&timeout=%s", url, name, timeout.String())
	return request, nil
}

// New function creates the configuration
func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error) {
	if len(config.ServiceUrl) == 0 {
		return nil, fmt.Errorf("serviceurl cannot be null")
	}

	if len(config.Name) == 0 {
		return nil, fmt.Errorf("name cannot be null")
	}

	timeout, err := time.ParseDuration(config.Timeout)

	if err != nil {
		return nil, err
	}

	request, err := buildRequest(config.ServiceUrl, config.Name, timeout)

	if err != nil {
		return nil, fmt.Errorf("error while building request")
	}

	return &Ondemand{
		next:    next,
		name:    config.Name,
		request: request,
		timeout: timeout,
	}, nil
}

// ServeHTTP retrieve the service status
func (e *Ondemand) ServeHTTP(rw http.ResponseWriter, req *http.Request) {

	log.Printf("Sending request: %s", e.request)
	status, err := getServiceStatus(e.request)
	log.Printf("Status: %s", status)

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
		rw.Write([]byte(pages.GetLoadingPage(e.name, e.timeout)))
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
