package sabliercmd

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/sablierapp/sablier/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// unsetNoColor removes NO_COLOR for the duration of the test. t.Setenv is
// called first so that its cleanup restores the original state, including
// leaving the variable unset when it was unset to begin with.
func unsetNoColor(t *testing.T) {
	t.Helper()
	t.Setenv("NO_COLOR", "")
	require.NoError(t, os.Unsetenv("NO_COLOR"))
}

func TestNewLoggerHonorsNoColor(t *testing.T) {
	// https://no-color.org: any NO_COLOR value other than the empty string
	// disables color, "regardless of its value".
	tests := []struct {
		name      string
		value     string
		unset     bool
		wantColor bool
	}{
		{name: "unset keeps color", unset: true, wantColor: true},
		{name: "empty keeps color", value: "", wantColor: true},
		{name: "1 disables color", value: "1", wantColor: false},
		{name: "0 disables color", value: "0", wantColor: false},
		{name: "arbitrary value disables color", value: "yes please", wantColor: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.unset {
				unsetNoColor(t)
			} else {
				t.Setenv("NO_COLOR", tt.value)
			}

			out := &bytes.Buffer{}
			newLogger(out, config.Logging{Level: "info"}).
				Info("connection established with docker", "provider", "docker")

			assert.Equal(t, tt.wantColor, strings.Contains(out.String(), "\x1b["),
				"unexpected ANSI escape sequences in %q", out.String())
			// The message and its attributes must survive either way.
			assert.Contains(t, out.String(), "connection established with docker")
			assert.Contains(t, out.String(), "docker")
		})
	}
}

func TestNoColor(t *testing.T) {
	t.Run("reports false when unset", func(t *testing.T) {
		unsetNoColor(t)
		assert.False(t, noColor())
	})

	t.Run("reports false when empty", func(t *testing.T) {
		t.Setenv("NO_COLOR", "")
		assert.False(t, noColor())
	})

	t.Run("reports true when set to any other value", func(t *testing.T) {
		for _, value := range []string{"1", "0", "true", "false", " "} {
			t.Setenv("NO_COLOR", value)
			assert.True(t, noColor(), "NO_COLOR=%q should disable color", value)
		}
	})
}
