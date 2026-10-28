package theme_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/theme"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func symlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("cannot create symlinks on this platform: %v", err)
	}
}

// kubeletVolume lays out files in dir the way the kubelet mounts a ConfigMap
// volume: the files live in a "..<timestamp>" directory, "..data" links to it,
// and every key is a symlink through "..data".
func kubeletVolume(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	const timestamped = "..2026_10_28_12_00_00.123456789"
	require.NoError(t, os.MkdirAll(filepath.Join(dir, timestamped), 0o755))
	for name, content := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, timestamped, name), []byte(content), 0o644))
	}
	// The target has to exist before the link is created: on Windows, that is
	// how os.Symlink tells a directory link from a file link.
	symlink(t, timestamped, filepath.Join(dir, "..data"))
	for name := range files {
		symlink(t, filepath.Join("..data", name), filepath.Join(dir, name))
	}
}

func render(t *testing.T, themes *theme.Themes, name string) string {
	t.Helper()
	w := &bytes.Buffer{}
	require.NoError(t, themes.Render(name, theme.Options{}, w))
	return w.String()
}

// TestNewWithCustomThemesFromPath_ConfigMapVolume: the themes of a ConfigMap
// volume are symlinks, which used to be dropped, so the only themes found were
// the ones in the timestamped directory, named after it.
func TestNewWithCustomThemesFromPath_ConfigMapVolume(t *testing.T) {
	dir := t.TempDir()
	kubeletVolume(t, dir, map[string]string{
		"brand.html": `<link rel="stylesheet" href="style.css">`,
		"style.css":  `body{color:red}`,
	})
	// A ConfigMap mounted below the themes directory is handled the same way.
	require.NoError(t, os.Mkdir(filepath.Join(dir, "team"), 0o755))
	kubeletVolume(t, filepath.Join(dir, "team"), map[string]string{
		"brand.html": `team`,
	})

	themes, err := theme.NewWithCustomThemesFromPath(dir, slogt.New(t))
	require.NoError(t, err)

	assert.Equal(t, []string{"brand", "ghost", "hacker-terminal", "matrix", "retro", "shuffle", "team/brand"}, themes.List())
	assert.Equal(t, `<style>body{color:red}</style>`, render(t, themes, "brand"))
	assert.Equal(t, `team`, render(t, themes, "team/brand"))
}

// TestNewWithCustomThemesFromPath_RefusesLinksOutside pins the reason symlinks
// used to be refused altogether: a link out of the themes directory must not
// let a theme inline a file from elsewhere on the host.
func TestNewWithCustomThemesFromPath_RefusesLinksOutside(t *testing.T) {
	outside := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outside, "secret.css"), []byte(`secret`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(outside, "secret.html"), []byte(`secret`), 0o644))

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "absolute.html"), []byte(`<link rel="stylesheet" href="abs/secret.css">`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "relative.html"), []byte(`<link rel="stylesheet" href="rel/secret.css">`), 0o644))
	symlink(t, outside, filepath.Join(dir, "abs"))
	rel, err := filepath.Rel(dir, outside)
	require.NoError(t, err)
	symlink(t, rel, filepath.Join(dir, "rel"))
	symlink(t, filepath.Join(outside, "secret.html"), filepath.Join(dir, "linked.html"))

	themes, err := theme.NewWithCustomThemesFromPath(dir, slogt.New(t))
	require.NoError(t, err, "a link outside the directory must be skipped, not fail every theme")

	assert.Equal(t, `<link rel="stylesheet" href="abs/secret.css">`, render(t, themes, "absolute"))
	assert.Equal(t, `<link rel="stylesheet" href="rel/secret.css">`, render(t, themes, "relative"))
	assert.False(t, themes.Exists("linked"))
}

func TestNewWithCustomThemesFromPath_SkipsLinkToDirectory(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "mine.html"), []byte(`mine`), 0o644))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "partials"), 0o755))
	symlink(t, "partials", filepath.Join(dir, "partials.html"))

	themes, err := theme.NewWithCustomThemesFromPath(dir, slogt.New(t))
	require.NoError(t, err)

	assert.True(t, themes.Exists("mine"))
	assert.False(t, themes.Exists("partials"))
}
