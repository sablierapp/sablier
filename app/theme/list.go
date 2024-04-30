package theme

import "strings"

// List all the loaded themes
func (t *Themes) List() []string {
	themes := make([]string, 0)

	for _, template := range t.themes.Templates() {
		if strings.HasSuffix(template.Name(), ".html") {
			themes = append(themes, strings.TrimSuffix(template.Name(), ".html"))
		}
	}

	return themes
}
