package main

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	jsoniter "github.com/json-iterator/tinygo"
	"github.com/tetratelabs/proxy-wasm-go-sdk/proxywasm"
	"github.com/tetratelabs/proxy-wasm-go-sdk/proxywasm/types"
	"golang.org/x/exp/slices"
)

var Version string

func main() {
	// SetVMContext is the entrypoint for setting up this entire Wasm VM.
	// Please make sure that this entrypoint be called during "main()" function, otherwise
	// this VM would fail.
	proxywasm.SetVMContext(&vmContext{})
}

// vmContext implements types.VMContext interface of proxy-wasm-go SDK.
type vmContext struct {
	// Embed the default VM context here,
	// so that we don't need to reimplement all the methods.
	types.DefaultVMContext
}

// Override types.DefaultVMContext.
func (*vmContext) NewPluginContext(contextID uint32) types.PluginContext {
	return &pluginContext{}
}

type pluginContext struct {
	// Embed the default plugin context here,
	// so that we don't need to reimplement all the methods.
	types.DefaultPluginContext
	configuration pluginConfiguration
}

type pluginConfiguration struct {
	cluster   string
	method    string
	path      string
	authority string
	timeout   uint32
}

// newPluginConfiguration creates a pluginConfiguration with default values
func newPluginConfiguration() pluginConfiguration {
	return pluginConfiguration{
		cluster:   "sablier:10000",
		method:    "GET",
		path:      "/",
		authority: "sablier.cluster.local",
		timeout:   5000, // timeout in milliseconds
	}
}

// Override types.DefaultPluginContext.
func (ctx *pluginContext) OnPluginStart(pluginConfigurationSize int) types.OnPluginStartStatus {
	proxywasm.LogInfof("sablier proxywasm plugin version %v loaded", Version)
	data, err := proxywasm.GetPluginConfiguration()
	if err != nil && err != types.ErrorStatusNotFound {
		proxywasm.LogCriticalf("error reading plugin configuration: %v", err)
		return types.OnPluginStartStatusFailed
	}

	proxywasm.LogInfof("plugin config: %s", string(data))
	config, err := parsePluginConfiguration(data)
	if err != nil {
		proxywasm.LogCriticalf("error parsing plugin configuration: %v", err)
		return types.OnPluginStartStatusFailed
	}

	ctx.configuration = config

	return types.OnPluginStartStatusOK
}

//go:generate go run github.com/json-iterator/tinygo/gen
type DynamicConfiguration struct {
	DisplayName      string `json:"display_name"`
	ShowDetails      *bool  `json:"show_details"`
	Theme            string `json:"theme"`
	RefreshFrequency string `json:"refresh_frequency"`
}

//go:generate go run github.com/json-iterator/tinygo/gen
type BlockingConfiguration struct {
	Timeout string `json:"timeout"`
}

//go:generate go run github.com/json-iterator/tinygo/gen
type Config struct {
	// SablierURL in the format of hostname:port. The scheme is excluded
	SablierURL string `json:"sablier_url"`
	// Cluster is an optional value that allows you to set override the
	// first argument to `proxywasm.DispatchHttpCall`.
	// In istio for exemple, the expected value would be: "outbound|port||hostname", e.g.: "outbound|10000||sablier"
	// In APISIX and Nginx for example, the value would be the same as SablierURL, e.g.: sablier:10000
	// Defaults to the same value of `SablierURL`.
	Cluster         string                 `json:"cluster"`
	Names           []string               `json:"names"`
	Group           string                 `json:"group"`
	SessionDuration string                 `json:"session_duration"`
	Dynamic         *DynamicConfiguration  `json:"dynamic"`
	Blocking        *BlockingConfiguration `json:"blocking"`
}

func (c Config) GetPath() string {
	path := url.URL{}
	q := path.Query()

	if c.SessionDuration != "" {
		dur, err := time.ParseDuration(c.SessionDuration)
		if err != nil {
			proxywasm.LogWarnf("parsing session duration failed (ignoring value): %v", err)
		} else {
			q.Add("session_duration", dur.String())
		}
	}

	for _, name := range c.Names {
		q.Add("names", name)
	}

	if c.Group != "" {
		q.Add("group", c.Group)
	}
	path.RawQuery = q.Encode()

	if c.Dynamic != nil {
		return c.getDynamicQuery(path)
	} else if c.Blocking != nil {
		return c.getBlockingQuery(path)
	}
	return "no strategy configured"
}

func (c Config) getDynamicQuery(path url.URL) string {
	path.Path = "/api/strategies/dynamic"
	q := path.Query()

	if c.Dynamic.DisplayName != "" {
		q.Add("display_name", c.Dynamic.DisplayName)
	}

	if c.Dynamic.Theme != "" {
		q.Add("theme", c.Dynamic.Theme)
	}

	if c.Dynamic.RefreshFrequency != "" {
		dur, err := time.ParseDuration(c.Dynamic.RefreshFrequency)
		if err != nil {
			proxywasm.LogWarnf("parsing dynamic refresh frequency failed (ignoring value): %v", err)
		} else {
			q.Add("refresh_frequency", dur.String())
		}
	}

	if c.Dynamic.ShowDetails != nil {
		q.Add("show_details", strconv.FormatBool(*c.Dynamic.ShowDetails))
	}
	path.RawQuery = q.Encode()

	return path.String()
}

func (c Config) getBlockingQuery(path url.URL) string {
	path.Path = "/api/strategies/blocking"
	q := path.Query()

	if c.Blocking.Timeout != "" {
		dur, err := time.ParseDuration(c.Blocking.Timeout)
		if err != nil {
			proxywasm.LogWarnf("parsing blocking timeout duration failed (ignoring value): %v", err)
		} else {
			q.Add("timeout", dur.String())
		}
	}
	path.RawQuery = q.Encode()

	return path.String()
}

func parsePluginConfiguration(data []byte) (pluginConfiguration, error) {
	pluginConf := newPluginConfiguration()
	if len(data) == 0 {
		return pluginConf, fmt.Errorf("the plugin configuration is not a valid: %q", string(data))
	}

	json := jsoniter.CreateJsonAdapter(Config_json{}, BlockingConfiguration_json{}, DynamicConfiguration_json{})

	var c Config
	err := json.Unmarshal(data, &c)
	if err != nil {
		proxywasm.LogErrorf("error parsing configuration: %v", err.Error())
		return pluginConf, err
	}

	if c.Blocking == nil && c.Dynamic == nil {
		return pluginConf, fmt.Errorf("you must specify one strategy (dynamic or blocking)")
	}

	if c.Blocking != nil && c.Dynamic != nil {
		return pluginConf, fmt.Errorf("you must specify only one strategy")
	}

	if len(c.Names) == 0 && len(c.Group) == 0 {
		return pluginConf, fmt.Errorf("you must specify names or group")
	}

	if len(c.Names) > 0 && len(c.Group) > 0 {
		return pluginConf, fmt.Errorf("you must specify either names or group")
	}

	if c.SablierURL != "" {
		pluginConf.authority = c.SablierURL

		// Default to SablierURL
		pluginConf.cluster = c.SablierURL
	}

	if c.Cluster != "" {
		pluginConf.cluster = c.Cluster
	}

	pluginConf.path = c.GetPath()

	return pluginConf, nil
}

// Override types.DefaultPluginContext.
func (ctx *pluginContext) NewHttpContext(contextID uint32) types.HttpContext {

	headers := [][2]string{
		{":method", ctx.configuration.method},
		{":path", ctx.configuration.path},
		{":authority", ctx.configuration.authority},
		{"User-Agent", fmt.Sprintf("sablier-proxywasm-plugin/%s", Version)},
	}
	return &httpOnDemand{
		contextID: contextID,
		headers:   headers,
		cluster:   ctx.configuration.cluster,
		timeout:   ctx.configuration.timeout,
	}
}

type httpOnDemand struct {
	// Embed the default http context here,
	// so that we don't need to reimplement all the methods.
	types.DefaultHttpContext
	contextID uint32
	headers   [][2]string
	cluster   string
	timeout   uint32
}

// Override types.DefaultHttpContext.
func (ctx *httpOnDemand) OnHttpRequestHeaders(numHeaders int, endOfStream bool) types.Action {
	proxywasm.LogInfof("DispatchHttpCall to %v", ctx.cluster)
	proxywasm.LogInfof("DispatchHttpCall with headers %v", ctx.headers)
	if _, err := proxywasm.DispatchHttpCall(ctx.cluster, ctx.headers, nil, nil,
		ctx.timeout, httpCallResponseCallback); err != nil {
		proxywasm.LogCriticalf("dipatch httpcall failed: %v", err)
		proxywasm.LogDebugf("%s: %v", ctx.cluster, ctx.headers)
		return types.ActionContinue
	}

	proxywasm.LogInfof("http call dispatched to %s", ctx.cluster)

	return types.ActionPause
}

func httpCallResponseCallback(numHeaders, bodySize, numTrailers int) {
	hs, err := proxywasm.GetHttpCallResponseHeaders()
	if err != nil {
		proxywasm.LogCriticalf("failed to get response headers: %v", err)
		return
	}

	proxywasm.LogInfof("GetHttpCallResponseHeaders: %v", hs)

	headerIndex := slices.IndexFunc(hs, func(h [2]string) bool { return strings.ToLower(h[0]) == "x-sablier-session-status" })
	if headerIndex < 0 {
		proxywasm.LogCriticalf("failed to find x-sablier-session-status header: %v", hs)
		proxywasm.ResumeHttpRequest()
		return
	}
	headerValue := hs[headerIndex][1]

	if headerValue != "ready" {
		b, err := proxywasm.GetHttpCallResponseBody(0, bodySize)
		if err != nil {
			proxywasm.LogCriticalf("failed to get response body: %v", err)
			proxywasm.ResumeHttpRequest()
			return
		}

		proxywasm.LogInfof("GetHttpCallResponseBody (%v bytes): %v", bodySize, string(b))

		if err := proxywasm.SendHttpResponse(200, hs, b, -1); err != nil {
			proxywasm.LogErrorf("failed to send local response: %v", err)
			proxywasm.ResumeHttpRequest()
		}
	} else {
		proxywasm.ResumeHttpRequest()
	}
}
