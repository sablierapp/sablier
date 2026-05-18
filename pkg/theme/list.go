package theme

import "strings"

// List all the loaded themes
func (t *Themes) List() []string {
	themes := make([]string, 0)

	for _, template := range t.themes.Templates() {
		if before, ok := strings.CutSuffix(template.Name(), ".html"); ok {
			themes = append(themes, before)
		}
	}

	return themes
}
