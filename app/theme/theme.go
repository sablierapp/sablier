package theme

import (
	"embed"
	"html/template"
	"io/fs"

	log "github.com/sirupsen/logrus"
)

// List of built-it themes
//
//go:embed embedded/*.html
var embeddedThemesFS embed.FS

type Themes struct {
	themes *template.Template
}

func New() (*Themes, error) {
	themes := &Themes{
		themes: template.New("root"),
	}

	err := ParseTemplatesFS(embeddedThemesFS, themes.themes)
	if err != nil {
		// Should never happen
		log.Errorf("could not parse embedded templates: %v", err)
		return nil, err
	}

	return themes, nil
}

func NewWithCustomThemes(custom fs.FS) (*Themes, error) {
	themes := &Themes{
		themes: template.New("root"),
	}

	err := ParseTemplatesFS(embeddedThemesFS, themes.themes)
	if err != nil {
		// Should never happen
		log.Errorf("could not parse embedded templates: %v", err)
		return nil, err
	}

	err = ParseTemplatesFS(custom, themes.themes)
	if err != nil {
		log.Errorf("could not parse custom templates: %v", err)
		return nil, err
	}

	return themes, nil
}
