package traefik_ondemand_plugin

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/acouvreur/traefik-ondemand-plugin/pkg/strategy"
)

// Config the plugin configuration
type Config struct {
	Name        string `yaml:"name"`
	ServiceUrl  string `yaml:"serviceurl"`
	Timeout     string `yaml:"timeout"`
	ErrorPage   string `yaml:"errorpage"`
	LoadingPage string `yaml:"loadingpage"`
	WaitUi      bool   `yaml:"waitUi"`
	BlockDelay  string `yaml:"blockDelay"`
}

// CreateConfig creates a config with its default values
func CreateConfig() *Config {
	return &Config{
		Timeout:     "1m",
		WaitUi:      true,
		BlockDelay:  "1m",
		ErrorPage:   "",
		LoadingPage: "",
	}
}

// Ondemand holds the request for the on demand service
type Ondemand struct {
	strategy strategy.Strategy
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

	strategy, err := config.getServeStrategy(request, name, next, timeout)

	if err != nil {
		return nil, err
	}

	return &Ondemand{
		strategy: strategy,
	}, nil
}

func (config *Config) getServeStrategy(request string, name string, next http.Handler, timeout time.Duration) (strategy.Strategy, error) {
	if config.WaitUi {
		return &strategy.DynamicStrategy{
			Request:     request,
			Name:        name,
			Next:        next,
			Timeout:     timeout,
			ErrorPage:   config.ErrorPage,
			LoadingPage: config.LoadingPage,
		}, nil
	} else {

		blockDelay, err := time.ParseDuration(config.BlockDelay)

		if err != nil {
			return nil, err
		}

		return &strategy.BlockingStrategy{
			Request:            request,
			Name:               name,
			Next:               next,
			Timeout:            timeout,
			BlockDelay:         blockDelay,
			BlockCheckInterval: 1 * time.Second,
		}, nil
	}
}

// ServeHTTP retrieve the service status
func (e *Ondemand) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	e.strategy.ServeHTTP(rw, req)
}
