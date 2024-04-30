package theme_test

import (
	"testing"
	"testing/fstest"

	"github.com/acouvreur/sablier/app/theme"
	"github.com/stretchr/testify/assert"
)

func TestList(t *testing.T) {
	themes, err := theme.NewWithCustomThemes(
		fstest.MapFS{
			"theme1.html":       &fstest.MapFile{},
			"inner/theme2.html": &fstest.MapFile{},
		})
	if err != nil {
		t.Error(err)
		return
	}

	list := themes.List()

	assert.ElementsMatch(t, []string{"theme1", "theme2", "ghost", "hacker-terminal", "matrix", "shuffle"}, list)
}
