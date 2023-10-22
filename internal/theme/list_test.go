package theme_test

import (
	"testing"
	"testing/fstest"

	"github.com/acouvreur/sablier/internal/theme"
	"github.com/stretchr/testify/assert"
)

func TestList(t *testing.T) {
	themes := theme.NewWithCustomThemes(
		fstest.MapFS{
			"theme1.html":       &fstest.MapFile{},
			"inner/theme2.html": &fstest.MapFile{},
		})

	list := themes.List()

	assert.ElementsMatch(t, []string{"theme1", "theme2", "ghost", "hacker-terminal", "matrix", "shuffle"}, list)
}
