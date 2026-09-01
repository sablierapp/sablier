package theme_test

import (
	"bytes"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/theme"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestList(t *testing.T) {
	themes, err := theme.NewWithCustomThemes(
		fstest.MapFS{
			"theme1.html":       &fstest.MapFile{},
			"inner/theme2.html": &fstest.MapFile{},
		}, slogt.New(t))
	if err != nil {
		t.Error(err)
		return
	}

	list := themes.List()

	// Nested themes are named after their path relative to the themes directory,
	// and the list is sorted so that the API returns a stable order.
	assert.Equal(t, []string{"ghost", "hacker-terminal", "inner/theme2", "matrix", "retro", "shuffle", "theme1"}, list)
}

func TestExists(t *testing.T) {
	themes, err := theme.NewWithCustomThemes(
		fstest.MapFS{
			"custom.html": &fstest.MapFile{},
		}, slogt.New(t))
	if err != nil {
		t.Error(err)
		return
	}

	assert.True(t, themes.Exists("custom"))
	assert.True(t, themes.Exists("ghost")) // embedded
	assert.False(t, themes.Exists("nope"))
}

// TestListDoesNotCollideOnBaseName covers the case reported in #1081: naming a
// theme after its base name let "special/secret.html" silently replace
// "secret.html", so one of the two themes was unreachable and never listed.
func TestListDoesNotCollideOnBaseName(t *testing.T) {
	themes, err := theme.NewWithCustomThemes(
		fstest.MapFS{
			"secret.html":         &fstest.MapFile{Data: []byte("root")},
			"special/secret.html": &fstest.MapFile{Data: []byte("nested")},
		}, slogt.New(t))
	if err != nil {
		t.Error(err)
		return
	}

	assert.Equal(t, []string{"ghost", "hacker-terminal", "matrix", "retro", "secret", "shuffle", "special/secret"}, themes.List())
	assert.True(t, themes.Exists("secret"))
	assert.True(t, themes.Exists("special/secret"))

	for name, want := range map[string]string{"secret": "root", "special/secret": "nested"} {
		w := &bytes.Buffer{}
		require.NoError(t, themes.Render(name, theme.Options{}, w))
		assert.Equal(t, want, w.String(), "theme %q rendered the wrong file", name)
	}
}

// TestListIsSorted pins the ordering of the list returned to the themes
// endpoint: the templates live in a map, so without an explicit sort the API
// returned a different order on every call.
func TestListIsSorted(t *testing.T) {
	themes, err := theme.NewWithCustomThemes(
		fstest.MapFS{
			"zulu.html":  &fstest.MapFile{},
			"alpha.html": &fstest.MapFile{},
			"mike.html":  &fstest.MapFile{},
		}, slogt.New(t))
	if err != nil {
		t.Error(err)
		return
	}

	want := themes.List()
	assert.True(t, slices.IsSorted(want))
	for range 10 {
		assert.Equal(t, want, themes.List())
	}
}

// TestNonThemeEntriesAreIgnored covers the second half of #1081: the walk used
// to accept anything whose path merely contained ".html", so a directory named
// "partials.html" was handed to the bundler, failed to be read, and took every
// custom theme down with it.
func TestNonThemeEntriesAreIgnored(t *testing.T) {
	themes, err := theme.NewWithCustomThemes(
		fstest.MapFS{
			"mine.html":                &fstest.MapFile{},
			"partials.html/header.txt": &fstest.MapFile{},
			"mine.html.bak":            &fstest.MapFile{},
		}, slogt.New(t))
	require.NoError(t, err)

	assert.True(t, themes.Exists("mine"))
	assert.False(t, themes.Exists("mine.html.bak"))
	assert.NotContains(t, themes.List(), "partials.html/header.txt")
}
