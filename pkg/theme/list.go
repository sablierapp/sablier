package theme

import (
	"slices"
	"strings"
)

// Exists reports whether a theme with the given name is loaded.
func (t *Themes) Exists(name string) bool {
	return t.themes.Lookup(name+".html") != nil
}

// List all the loaded themes, sorted by name.
//
// The order is explicit because the underlying templates are held in a map:
// without sorting, the themes endpoint and the "theme not found" problem detail
// returned a differently ordered list on every call.
func (t *Themes) List() []string {
	themes := make([]string, 0)

	for _, template := range t.themes.Templates() {
		if before, ok := strings.CutSuffix(template.Name(), ".html"); ok {
			themes = append(themes, before)
		}
	}

	slices.Sort(themes)
	return themes
}
