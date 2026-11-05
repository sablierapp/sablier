package sabliercmd_test

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/sablierapp/sablier/pkg/config"
	"github.com/sablierapp/sablier/pkg/sabliercmd"
	"github.com/stretchr/testify/require"
)

func executeWebhooks(t *testing.T, args ...string) (config.Webhooks, error) {
	t.Helper()
	sabliercmd.SetStartCommand(mockStartCommand)
	t.Cleanup(sabliercmd.ResetStartCommand)

	sabliercmd.ResetConfig()
	cmd := sabliercmd.NewRootCommand()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return sabliercmd.GetConfig().Webhooks, err
}

func TestWebhooksFromEnv(t *testing.T) {
	t.Setenv("SABLIER_WEBHOOKS_ENDPOINTS_0_URL", "https://env.example.com/0")
	t.Setenv("SABLIER_WEBHOOKS_ENDPOINTS_0_EVENTS", "started, stopped")
	t.Setenv("SABLIER_WEBHOOKS_ENDPOINTS_0_HEADERS_AUTHORIZATION", "Bearer env")
	t.Setenv("SABLIER_WEBHOOKS_ENDPOINTS_0_HEADERS_X_API_KEY", "key")
	t.Setenv("SABLIER_WEBHOOKS_ENDPOINTS_1_URL", "https://env.example.com/1")

	got, err := executeWebhooks(t, "start")
	require.NoError(t, err)

	require.Equal(t, config.Webhooks{Endpoints: []config.WebhookEndpoint{
		{
			URL:     "https://env.example.com/0",
			Headers: map[string]string{"Authorization": "Bearer env", "X-Api-Key": "key"},
			Events:  []string{"started", "stopped"},
		},
		{URL: "https://env.example.com/1"},
	}}, got)
}

func TestWebhooksFromFlags(t *testing.T) {
	got, err := executeWebhooks(t,
		"start",
		"--webhooks.endpoints[0].url=https://cli.example.com/0",
		"--webhooks.endpoints[0].events=stopped",
		"--webhooks.endpoints[0].headers.Authorization", "Bearer cli",
		"--webhooks.endpoints[1].url", "https://cli.example.com/1",
	)
	require.NoError(t, err)

	require.Equal(t, config.Webhooks{Endpoints: []config.WebhookEndpoint{
		{
			URL:     "https://cli.example.com/0",
			Headers: map[string]string{"Authorization": "Bearer cli"},
			Events:  []string{"stopped"},
		},
		{URL: "https://cli.example.com/1"},
	}}, got)
}

func TestWebhooksPrecedence(t *testing.T) {
	testDir, err := os.Getwd()
	require.NoError(t, err)

	// testdata/config.yml defines endpoint 0 with an Authorization header and both events.
	t.Setenv("SABLIER_WEBHOOKS_ENDPOINTS_0_URL", "https://env.example.com/0")
	t.Setenv("SABLIER_WEBHOOKS_ENDPOINTS_0_EVENTS", "started")
	t.Setenv("SABLIER_WEBHOOKS_ENDPOINTS_1_URL", "https://env.example.com/1")

	got, err := executeWebhooks(t,
		"--configFile", filepath.Join(testDir, "testdata", "config.yml"),
		"start",
		"--webhooks.endpoints[0].url=https://cli.example.com/0",
		"--webhooks.endpoints[0].headers.Authorization=Bearer cli",
	)
	require.NoError(t, err)

	require.Equal(t, config.Webhooks{Endpoints: []config.WebhookEndpoint{
		{
			URL:     "https://cli.example.com/0",
			Headers: map[string]string{"Authorization": "Bearer cli"},
			Events:  []string{"started"},
		},
		{URL: "https://env.example.com/1"},
	}}, got)
}

func TestWebhooksFromEnvAndFlags(t *testing.T) {
	t.Setenv("SABLIER_WEBHOOKS_ENDPOINTS_1_URL", "https://env.example.com/1")

	got, err := executeWebhooks(t, "start", "--webhooks.endpoints[0].url=https://cli.example.com/0")
	require.NoError(t, err)

	require.Equal(t, config.Webhooks{Endpoints: []config.WebhookEndpoint{
		{URL: "https://cli.example.com/0"},
		{URL: "https://env.example.com/1"},
	}}, got)
}

func TestWebhooksInvalid(t *testing.T) {
	t.Run("index gap", func(t *testing.T) {
		t.Setenv("SABLIER_WEBHOOKS_ENDPOINTS_1_URL", "https://env.example.com/1")

		_, err := executeWebhooks(t, "start")
		require.ErrorContains(t, err, "webhooks.endpoints[1]")
	})

	t.Run("missing url", func(t *testing.T) {
		_, err := executeWebhooks(t, "start", "--webhooks.endpoints[0].events=started")
		require.ErrorContains(t, err, "webhooks.endpoints[0].url is required")
	})

	t.Run("unknown field", func(t *testing.T) {
		_, err := executeWebhooks(t, "start", "--webhooks.endpoints[0].uri=https://cli.example.com")
		require.ErrorContains(t, err, "unknown flag")
	})
}
