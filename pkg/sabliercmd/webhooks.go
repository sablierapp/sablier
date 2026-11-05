package sabliercmd

import (
	"fmt"
	"maps"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/sablierapp/sablier/pkg/config"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var (
	// webhookFlagPattern matches indexed webhook flags such as
	// webhooks.endpoints[0].url or webhooks.endpoints[1].headers.Authorization.
	webhookFlagPattern = regexp.MustCompile(`^webhooks\.endpoints\[(\d+)\]\.(url|events|headers\.(.+))$`)
	// webhookEnvPattern matches indexed webhook environment variables such as
	// SABLIER_WEBHOOKS_ENDPOINTS_0_URL or SABLIER_WEBHOOKS_ENDPOINTS_1_HEADERS_X_API_KEY.
	webhookEnvPattern = regexp.MustCompile(`^SABLIER_WEBHOOKS_ENDPOINTS_(\d+)_(URL|EVENTS|HEADERS_(.+))$`)
)

// webhookEndpointOverride holds the fields of one endpoint set by environment
// variables or flags. A nil field was not set.
type webhookEndpointOverride struct {
	url     *string
	events  []string
	headers map[string]string
}

// acceptWebhookFlags lets flags accept the indexed webhook flags. Their names
// are not known in advance, so each one is defined when the parser first looks
// it up.
func acceptWebhookFlags(flags *pflag.FlagSet) {
	defining := false
	flags.SetNormalizeFunc(func(f *pflag.FlagSet, name string) pflag.NormalizedName {
		if defining {
			return pflag.NormalizedName(name)
		}
		m := webhookFlagPattern.FindStringSubmatch(name)
		if m == nil {
			return pflag.NormalizedName(name)
		}
		defining = true
		defer func() { defining = false }()
		if f.Lookup(name) == nil {
			if m[2] == "events" {
				f.StringSlice(name, nil, "Webhook endpoint events")
			} else {
				f.String(name, "", "Webhook endpoint "+m[2])
			}
		}
		return pflag.NormalizedName(name)
	})
}

// loadWebhooks reads the webhook endpoints from the configuration file, then
// applies the indexed environment variables and flags on top, in that order of
// precedence.
func loadWebhooks(cmd *cobra.Command, v *viper.Viper) (config.Webhooks, error) {
	var webhooks config.Webhooks
	if err := v.UnmarshalKey("webhooks", &webhooks); err != nil {
		return webhooks, err
	}

	// Flags are collected after environment variables so they win field by field.
	overrides := map[int]*webhookEndpointOverride{}
	if err := webhookEnvOverrides(overrides, os.Environ()); err != nil {
		return webhooks, err
	}
	if err := webhookFlagOverrides(overrides, cmd.Flags()); err != nil {
		return webhooks, err
	}
	var err error
	if webhooks.Endpoints, err = applyWebhookOverrides(webhooks.Endpoints, overrides); err != nil {
		return webhooks, err
	}

	for i, ep := range webhooks.Endpoints {
		if ep.URL == "" {
			return webhooks, fmt.Errorf("webhooks.endpoints[%d].url is required", i)
		}
	}
	return webhooks, nil
}

func webhookEnvOverrides(overrides map[int]*webhookEndpointOverride, environ []string) error {
	for _, kv := range environ {
		key, value, _ := strings.Cut(kv, "=")
		m := webhookEnvPattern.FindStringSubmatch(key)
		if m == nil {
			continue
		}
		o, err := webhookOverrideAt(overrides, m[1])
		if err != nil {
			return fmt.Errorf("%s: %w", key, err)
		}
		switch m[2] {
		case "URL":
			o.url = &value
		case "EVENTS":
			o.events = splitEvents(value)
		default:
			// Environment variable names cannot hold "-", so X_API_KEY stands for X-Api-Key.
			o.setHeader(http.CanonicalHeaderKey(strings.ReplaceAll(m[3], "_", "-")), value)
		}
	}
	return nil
}

func webhookFlagOverrides(overrides map[int]*webhookEndpointOverride, flags *pflag.FlagSet) error {
	var err error
	flags.Visit(func(f *pflag.Flag) {
		m := webhookFlagPattern.FindStringSubmatch(f.Name)
		if m == nil || err != nil {
			return
		}
		var o *webhookEndpointOverride
		if o, err = webhookOverrideAt(overrides, m[1]); err != nil {
			err = fmt.Errorf("--%s: %w", f.Name, err)
			return
		}
		switch m[2] {
		case "url":
			value := f.Value.String()
			o.url = &value
		case "events":
			o.events, _ = flags.GetStringSlice(f.Name)
		default:
			o.setHeader(m[3], f.Value.String())
		}
	})
	return err
}

func webhookOverrideAt(overrides map[int]*webhookEndpointOverride, index string) (*webhookEndpointOverride, error) {
	i, err := strconv.Atoi(index)
	if err != nil {
		return nil, fmt.Errorf("invalid endpoint index %q", index)
	}
	o, ok := overrides[i]
	if !ok {
		o = &webhookEndpointOverride{}
		overrides[i] = o
	}
	return o, nil
}

// applyWebhookOverrides sets the overridden fields on the existing endpoints.
// An override for the index right after the last endpoint adds a new one.
func applyWebhookOverrides(endpoints []config.WebhookEndpoint, overrides map[int]*webhookEndpointOverride) ([]config.WebhookEndpoint, error) {
	for _, i := range slices.Sorted(maps.Keys(overrides)) {
		if i > len(endpoints) {
			return nil, fmt.Errorf("webhooks.endpoints[%d] is set but webhooks.endpoints[%d] is not", i, len(endpoints))
		}
		if i == len(endpoints) {
			endpoints = append(endpoints, config.WebhookEndpoint{})
		}
		o := overrides[i]
		ep := &endpoints[i]
		if o.url != nil {
			ep.URL = *o.url
		}
		if o.events != nil {
			ep.Events = o.events
		}
		for name, value := range o.headers {
			if ep.Headers == nil {
				ep.Headers = map[string]string{}
			}
			setHeader(ep.Headers, name, value)
		}
	}
	return endpoints, nil
}

func (o *webhookEndpointOverride) setHeader(name, value string) {
	if o.headers == nil {
		o.headers = map[string]string{}
	}
	setHeader(o.headers, name, value)
}

// setHeader sets a header, replacing any key that differs only in case, since
// header names are case-insensitive (the configuration file lower-cases them).
func setHeader(headers map[string]string, name, value string) {
	for k := range headers {
		if strings.EqualFold(k, name) {
			delete(headers, k)
		}
	}
	headers[name] = value
}

func splitEvents(value string) []string {
	events := []string{}
	for event := range strings.SplitSeq(value, ",") {
		if event = strings.TrimSpace(event); event != "" {
			events = append(events, event)
		}
	}
	return events
}
