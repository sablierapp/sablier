package sabliercmd

import (
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/sablierapp/sablier/pkg/config"
	"github.com/sablierapp/sablier/pkg/provider/providertest"
	"go.uber.org/mock/gomock"
	"gotest.tools/v3/assert"
)

// activityProvider is a provider that supports activity tracking. The mocks
// have no expectations: setupActivity must not call the provider.
type activityProvider struct {
	*providertest.MockProvider
	*providertest.MockActivityHost
}

func newActivityProvider(t *testing.T) activityProvider {
	ctrl := gomock.NewController(t)
	return activityProvider{
		MockProvider:     providertest.NewMockProvider(ctrl),
		MockActivityHost: providertest.NewMockActivityHost(ctrl),
	}
}

func assertActivityError(t *testing.T, err error, contains ...string) {
	t.Helper()
	assert.Assert(t, err != nil, "setupActivity must fail")
	assert.Assert(t, strings.HasPrefix(err.Error(), "--activity.enabled: "), err.Error())
	for _, c := range contains {
		assert.ErrorContains(t, err, c)
	}
}

func TestSetupActivity_ProviderWithoutActivityHost(t *testing.T) {
	p := providertest.NewMockProvider(gomock.NewController(t))
	conf := config.Activity{Enabled: true, Interval: 20 * time.Second}

	src, closer, err := setupActivity(conf, "kubernetes", p, slog.New(slog.DiscardHandler))

	assertActivityError(t, err, "does not support activity tracking", `"kubernetes"`)
	assert.Assert(t, src == nil)
	assert.Assert(t, closer == nil)
}

func TestSetupActivity_InvalidCIDR(t *testing.T) {
	conf := config.Activity{Enabled: true, Interval: 20 * time.Second, IgnoreCIDRs: []string{"10.0.0.0/8", "nope"}}

	src, closer, err := setupActivity(conf, "docker", newActivityProvider(t), slog.New(slog.DiscardHandler))

	assertActivityError(t, err, "--activity.ignore-cidrs", `"nope"`)
	assert.Assert(t, src == nil)
	assert.Assert(t, closer == nil)
}

func TestSetupActivity_InvalidInterval(t *testing.T) {
	for _, interval := range []time.Duration{0, -time.Second} {
		t.Run(interval.String(), func(t *testing.T) {
			conf := config.Activity{Enabled: true, Interval: interval}

			src, closer, err := setupActivity(conf, "docker", newActivityProvider(t), slog.New(slog.DiscardHandler))

			assertActivityError(t, err, "--activity.interval", "greater than 0")
			assert.Assert(t, src == nil)
			assert.Assert(t, closer == nil)
		})
	}
}
