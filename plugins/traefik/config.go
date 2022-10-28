package traefik

import (
	"fmt"
	"net/http"
	"strings"
)

type DynamicConfiguration struct {
	DisplayName string `yaml:"displayname"`
	Theme       string `yaml:"theme"`
}

type BlockingConfiguration struct {
	Timeout string `yaml:"timeout"`
}

type Config struct {
	SablierURL      string `yaml:"sablierUrl"`
	Names           string `yaml:"names"`
	SessionDuration string `yaml:"sessionDuration"`
	splittedNames   []string
	Dynamic         *DynamicConfiguration  `yaml:"dynamic"`
	Blocking        *BlockingConfiguration `yaml:"blocking"`
}

func CreateConfig() *Config {
	return &Config{
		SablierURL:      "http://sablier:10000",
		Names:           "",
		SessionDuration: "",
		splittedNames:   []string{},
		Dynamic:         nil,
		Blocking:        nil,
	}
}

func (c *Config) BuildRequest(middlewareName string) (*http.Request, error) {

	if len(c.SablierURL) == 0 {
		return nil, fmt.Errorf("sablierURL cannot be empty")
	}

	names := strings.Split(c.Names, ",")
	for i := range names {
		names[i] = strings.TrimSpace(names[i])
	}

	c.splittedNames = names

	if len(names) == 0 {
		return nil, fmt.Errorf("you must specify at least one name")
	}

	if c.Dynamic != nil && c.Blocking != nil {
		return nil, fmt.Errorf("only supply one strategy: dynamic or blocking")
	}

	if c.Dynamic != nil {
		return c.buildDynamicRequest(middlewareName)
	} else if c.Blocking != nil {
		return c.buildBlockingRequest()
	}
	return nil, fmt.Errorf("no strategy configured")
}

func (c *Config) buildDynamicRequest(middlewareName string) (*http.Request, error) {
	if c.Dynamic == nil {
		return nil, fmt.Errorf("dynamic config is nil")
	}

	request, err := http.NewRequest("GET", fmt.Sprintf("%s/api/strategies/dynamic", c.SablierURL), nil)
	if err != nil {
		return nil, err
	}

	q := request.URL.Query()

	q.Add("session_duration", c.SessionDuration)
	for _, name := range c.splittedNames {
		q.Add("names", name)
	}

	if c.Dynamic.DisplayName != "" {
		q.Add("display_name", c.Dynamic.DisplayName)
	} else {
		// display name defaults as middleware name
		q.Add("display_name", middlewareName)
	}

	if c.Dynamic.Theme != "" {
		q.Add("theme", c.Dynamic.Theme)
	}

	request.URL.RawQuery = q.Encode()

	return request, nil
}

func (c *Config) buildBlockingRequest() (*http.Request, error) {
	if c.Blocking == nil {
		return nil, fmt.Errorf("blocking config is nil")
	}

	request, err := http.NewRequest("GET", fmt.Sprintf("%s/api/strategies/blocking", c.SablierURL), nil)
	if err != nil {
		return nil, err
	}

	q := request.URL.Query()

	q.Add("session_duration", c.SessionDuration)
	for _, name := range c.splittedNames {
		q.Add("names", name)
	}

	if c.Blocking.Timeout != "" {
		q.Add("timeout", c.Blocking.Timeout)
	}

	request.URL.RawQuery = q.Encode()

	return request, nil
}
