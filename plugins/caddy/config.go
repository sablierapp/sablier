package caddy

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
)

func init() {
	httpcaddyfile.RegisterHandlerDirective("sablier", parseCaddyfile)

}

type DynamicConfiguration struct {
	DisplayName      string
	ShowDetails      *bool
	Theme            string
	RefreshFrequency *time.Duration
}

type BlockingConfiguration struct {
	Timeout          *time.Duration
	RefreshFrequency *time.Duration
}

type Config struct {
	SablierURL      string
	Names           []string
	Group           string
	SessionDuration *time.Duration
	Dynamic         *DynamicConfiguration
	Blocking        *BlockingConfiguration
}

func CreateConfig() *Config {
	return &Config{
		SablierURL:      "http://sablier:10000",
		Names:           []string{},
		SessionDuration: nil,
		Dynamic:         nil,
		Blocking:        nil,
	}
}

// UnmarshalCaddyfile implements caddyfile.Unmarshaler. Syntax:
//
//	sablier [<sablierURL>] {
//		[names container1,container2,...]
//		[group mygroup]
//		[session_duration 30m]
//		dynamic {
//			[display_name This is my display name]
//			[show_details yes|true|on]
//			[theme hacker-terminal]
//			[refresh_frequency 2s]
//		}
//		blocking {
//			[timeout 1m]
//		}
//	}
func (c *Config) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	for d.Next() {
		if d.NextArg() {
			c.SablierURL = d.Val()
		} else {
			c.SablierURL = "http://sablier:10000"
		}
		if d.NextArg() {
			return d.ArgErr()
		}
		for nesting := d.Nesting(); d.NextBlock(nesting); {
			subdirective := d.Val()
			args := strings.Join(d.RemainingArgs(), " ")
			switch subdirective {
			case "names":
				c.Names = parseNames(args)
			case "group":
				c.Group = args
			case "session_duration":
				duration, err := time.ParseDuration(args)
				if err != nil {
					return err
				}
				c.SessionDuration = &duration
			case "dynamic":
				dynamic, err := parseDynamic(d)
				if err != nil {
					return err
				}
				c.Dynamic = dynamic
			case "blocking":
				blocking, err := parseBlocking(d)
				if err != nil {
					return err
				}
				c.Blocking = blocking
			}
		}
	}

	if c.Blocking == nil && c.Dynamic == nil {
		return fmt.Errorf("you must specify one strategy (dynamic or blocking)")
	}

	if c.Blocking != nil && c.Dynamic != nil {
		return fmt.Errorf("you must specify only one strategy")
	}

	if len(c.Names) == 0 && len(c.Group) == 0 {
		return fmt.Errorf("you must specify names or group")
	}

	if len(c.Names) > 0 && len(c.Group) > 0 {
		return fmt.Errorf("you must specify either names or group")
	}

	return nil
}

func parseNames(value string) []string {
	names := strings.Split(value, " ")
	for i := range names {
		names[i] = strings.TrimSpace(names[i])
	}

	if len(names) == 1 && names[0] == "" {
		return make([]string, 0)
	}

	return names
}

func parseDynamic(d *caddyfile.Dispenser) (*DynamicConfiguration, error) {
	conf := &DynamicConfiguration{}
	for nesting := d.Nesting(); d.NextBlock(nesting); {
		subdirective := d.Val()
		args := strings.Join(d.RemainingArgs(), " ")
		switch subdirective {
		case "display_name":
			conf.DisplayName = args
		case "show_details":
			shouldShow := isEnabledArg(args)
			conf.ShowDetails = &shouldShow
		case "theme":
			conf.Theme = args
		case "refresh_frequency":
			duration, err := time.ParseDuration(args)
			if err != nil {
				return nil, err
			}
			conf.RefreshFrequency = &duration
		}
	}
	return conf, nil
}

func parseBlocking(d *caddyfile.Dispenser) (*BlockingConfiguration, error) {
	conf := &BlockingConfiguration{}
	for nesting := d.Nesting(); d.NextBlock(nesting); {
		subdirective := d.Val()
		args := strings.Join(d.RemainingArgs(), " ")
		switch subdirective {
		case "timeout":
			duration, err := time.ParseDuration(args)
			if err != nil {
				return nil, err
			}
			conf.Timeout = &duration
		case "refresh_frequency":
			duration, err := time.ParseDuration(args)
			if err != nil {
				return nil, err
			}
			conf.RefreshFrequency = &duration
		}
	}
	return conf, nil
}

func parseCaddyfile(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	var c Config
	err := c.UnmarshalCaddyfile(h.Dispenser)
	return SablierMiddleware{Config: c}, err
}

func isEnabledArg(s string) bool {
	if s == "yes" || s == "true" || s == "on" {
		return true
	}
	return false
}

func (c *Config) BuildRequest() (*http.Request, error) {
	if c.Dynamic != nil {
		return c.buildDynamicRequest()
	} else if c.Blocking != nil {
		return c.buildBlockingRequest()
	}
	return nil, fmt.Errorf("no strategy configured")
}

func (c *Config) buildDynamicRequest() (*http.Request, error) {
	if c.Dynamic == nil {
		return nil, fmt.Errorf("dynamic config is nil")
	}

	request, err := http.NewRequest("GET", fmt.Sprintf("%s/api/strategies/dynamic", c.SablierURL), nil)
	if err != nil {
		return nil, err
	}

	q := request.URL.Query()

	if c.SessionDuration != nil {
		q.Add("session_duration", c.SessionDuration.String())
	}

	for _, name := range c.Names {
		q.Add("names", name)
	}

	if c.Group != "" {
		q.Add("group", c.Group)
	}

	if c.Dynamic.DisplayName != "" {
		q.Add("display_name", c.Dynamic.DisplayName)
	}

	if c.Dynamic.Theme != "" {
		q.Add("theme", c.Dynamic.Theme)
	}

	if c.Dynamic.RefreshFrequency != nil {
		q.Add("refresh_frequency", c.Dynamic.RefreshFrequency.String())
	}

	if c.Dynamic.ShowDetails != nil {
		q.Add("show_details", strconv.FormatBool(*c.Dynamic.ShowDetails))
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

	if c.SessionDuration != nil {
		q.Add("session_duration", c.SessionDuration.String())
	}

	for _, name := range c.Names {
		q.Add("names", name)
	}

	if c.Group != "" {
		q.Add("group", c.Group)
	}

	if c.Blocking.Timeout != nil {
		q.Add("timeout", c.Blocking.Timeout.String())
	}

	request.URL.RawQuery = q.Encode()

	return request, nil
}
