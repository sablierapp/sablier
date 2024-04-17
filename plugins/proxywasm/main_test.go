package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tetratelabs/proxy-wasm-go-sdk/proxywasm/proxytest"
	"github.com/tetratelabs/proxy-wasm-go-sdk/proxywasm/types"
)

func TestUnmarshal(t *testing.T) {
	data := `{
		"sablier_url": "sablier",
		"sablier_port": 10000,
		"group": "demo",
		"session_duration": "30s",
		"dynamic": {
		  "display_dame": "From WASM!",
		  "show_details": true,
		  "theme": "hacker-terminal",
		  "refresh_frequency": "5s"
		}
	  }`

	config, err := parsePluginConfiguration([]byte(data))

	if err != nil {
		t.Error(err)
	}

	t.Log("path:", config.path)
	t.Log("authority:", config.authority)
}

func TestPluginContext_OnTick(t *testing.T) {
	vmTest(t, func(t *testing.T, vm types.VMContext) {
		data := `{
			"sablier_url": "sablier",
			"sablier_port": 10000,
			"group": "demo",
			"session_duration": "30s",
			"dynamic": {
			  "display_dame": "From WASM!",
			  "show_details": true,
			  "theme": "hacker-terminal",
			  "refresh_frequency": "5s"
			}
		  }`
		opt := proxytest.NewEmulatorOption().WithVMContext(vm).WithPluginConfiguration([]byte(data))
		host, reset := proxytest.NewHostEmulator(opt)
		defer reset()

		// Create http context.
		id := host.InitializeHttpContext()

		// Call OnRequestHeaders.
		action := host.CallOnRequestHeaders(id, [][2]string{
			{"content-length", "10"},
		}, false)

		// Must be continued.
		require.Equal(t, types.ActionPause, action)

		// Check the final request headers
		host.CallOnHttpCallResponse(id, [][2]string{
			{"x-sablier-session-status", "not-ready"},
		}, nil, []byte("Response from Sablier"))
		response := host.GetCurrentResponseBody(id)
		require.Equal(t,
			"Response from Sablier",
			response,
			"response should be served from sablier.")
	})
}

// vmTest executes f twice, once with a types.VMContext that executes plugin code directly
// in the host, and again by executing the plugin code within the compiled main.wasm binary.
// Execution with main.wasm will be skipped if the file cannot be found.
func vmTest(t *testing.T, f func(*testing.T, types.VMContext)) {
	t.Helper()

	t.Run("go", func(t *testing.T) {
		f(t, &vmContext{})
	})

	t.Run("wasm", func(t *testing.T) {
		wasm, err := os.ReadFile("sablierproxywasm.wasm")
		if err != nil {
			t.Skip("wasm not found")
		}
		v, err := proxytest.NewWasmVMContext(wasm)
		require.NoError(t, err)
		defer v.Close()
		f(t, v)
	})
}
