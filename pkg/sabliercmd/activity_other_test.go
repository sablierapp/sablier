//go:build !(linux && (amd64 || arm64))

package sabliercmd

import (
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/sablierapp/sablier/pkg/activity/bpf"
	"github.com/sablierapp/sablier/pkg/config"
	"gotest.tools/v3/assert"
)

func TestSetupActivity_Unsupported(t *testing.T) {
	conf := config.Activity{Enabled: true, Interval: 20 * time.Second, IgnoreCIDRs: []string{"192.168.0.2/32"}}

	src, closer, err := setupActivity(conf, "docker", newActivityProvider(t), slog.New(slog.DiscardHandler))

	assertActivityError(t, err)
	assert.Assert(t, errors.Is(err, bpf.ErrNotSupported), err.Error())
	assert.Assert(t, src == nil)
	assert.Assert(t, closer == nil)
}
