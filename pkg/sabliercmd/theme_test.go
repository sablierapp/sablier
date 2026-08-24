package sabliercmd

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/sablierapp/sablier/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestLogger returns a debug-level logger writing into buf so the tests can
// assert on the severity used to report a missing custom themes directory.
func newTestLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func confWithThemesPath(path string) config.Config {
	c := config.NewConfig()
	c.Strategy.Dynamic.CustomThemesPath = path
	return c
}

func TestSetupTheme(t *testing.T) {
	t.Run("default path is the documented mount point", func(t *testing.T) {
		assert.Equal(t, "/etc/sablier/themes", config.NewConfig().Strategy.Dynamic.CustomThemesPath)
	})

	t.Run("missing directory keeps the embedded themes without warning", func(t *testing.T) {
		buf := &bytes.Buffer{}
		// The default path does not exist on the machine running the tests, which is
		// exactly the situation every deployment without custom themes is in.
		missing := filepath.Join(t.TempDir(), "does-not-exist")

		themes, err := setupTheme(context.Background(), confWithThemesPath(missing), newTestLogger(buf))
		require.NoError(t, err)

		assert.ElementsMatch(t, []string{"ghost", "hacker-terminal", "matrix", "shuffle"}, themes.List())
		assert.NotContains(t, buf.String(), "level=WARN")
	})

	t.Run("custom themes are loaded from the directory", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "mine.html"), []byte("<html></html>"), 0o600))

		themes, err := setupTheme(context.Background(), confWithThemesPath(dir), newTestLogger(&bytes.Buffer{}))
		require.NoError(t, err)

		assert.True(t, themes.Exists("mine"))
	})

	t.Run("an unreadable directory is still reported as a warning", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root bypasses directory permissions")
		}
		dir := filepath.Join(t.TempDir(), "unreadable")
		require.NoError(t, os.Mkdir(dir, 0o000))
		t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

		buf := &bytes.Buffer{}
		themes, err := setupTheme(context.Background(), confWithThemesPath(dir), newTestLogger(buf))
		require.NoError(t, err)

		assert.True(t, themes.Exists("hacker-terminal"))
		assert.Contains(t, buf.String(), "level=WARN")
	})

	t.Run("an empty path skips the lookup entirely", func(t *testing.T) {
		buf := &bytes.Buffer{}
		themes, err := setupTheme(context.Background(), confWithThemesPath(""), newTestLogger(buf))
		require.NoError(t, err)

		assert.ElementsMatch(t, []string{"ghost", "hacker-terminal", "matrix", "shuffle"}, themes.List())
		assert.Contains(t, buf.String(), "--strategy.dynamic.custom-themes-path is empty")
	})
}
