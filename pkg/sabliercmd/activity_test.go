package sabliercmd

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sablierapp/sablier/pkg/config"
	"github.com/sablierapp/sablier/pkg/provider/podman"
	"github.com/sablierapp/sablier/pkg/provider/providertest"
	"github.com/sablierapp/sablier/pkg/sablier"
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

// Podman embeds the Docker provider, so it implements sablier.ActivityHost, but
// its containers' cgroups aren't where the Docker provider computes them.
func TestSetupActivity_PodmanIsRejected(t *testing.T) {
	var p sablier.Provider = &podman.Provider{}
	_, isHost := p.(sablier.ActivityHost)
	assert.Assert(t, isHost, "the test needs a provider that implements sablier.ActivityHost")
	conf := config.Activity{Enabled: true, Interval: 20 * time.Second}

	src, closer, err := setupActivity(conf, "podman", p, slog.New(slog.DiscardHandler))

	assertActivityError(t, err, `provider "podman"`, "activity tracking is Docker-only")
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

func TestWarnPrivateCgroupNamespace(t *testing.T) {
	tests := []struct {
		name     string
		content  *string // nil: no file
		wantWarn bool
	}{
		{name: "private namespace", content: new("0::/\n"), wantWarn: true},
		{name: "host namespace, systemd driver", content: new("0::/system.slice/docker-0123abcd.scope\n")},
		{name: "host namespace, cgroupfs driver", content: new("0::/docker/0123abcd\n")},
		{name: "cgroup v1", content: new("12:pids:/\n11:memory:/\n0::/\n")},
		{name: "no file"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "cgroup")
			if tt.content != nil {
				assert.NilError(t, os.WriteFile(path, []byte(*tt.content), 0o600))
			}
			var logs bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&logs, nil))

			warnPrivateCgroupNamespace(logger, path)

			if !tt.wantWarn {
				assert.Equal(t, logs.String(), "")
				return
			}
			assert.Assert(t, strings.Contains(logs.String(), "level=WARN"), logs.String())
			assert.Assert(t, strings.Contains(logs.String(), "Sablier appears to run in a private cgroup namespace; activity tracking needs cgroup: host"), logs.String())
		})
	}
}
