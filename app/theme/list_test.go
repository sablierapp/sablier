package theme_test

import (
	"github.com/neilotoole/slogt"
	"testing"
	"testing/fstest"

	"github.com/sablierapp/sablier/app/theme"
	"github.com/stretchr/testify/assert"
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

	assert.ElementsMatch(t, []string{"theme1", "theme2", "ghost", "hacker-terminal", "matrix", "shuffle"}, list)
}
