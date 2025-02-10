package theme

import (
	"embed"
	"html/template"
	"io/fs"
	"log/slog"
)

// List of built-it themes
//
//go:embed embedded/*.html
var embeddedThemesFS embed.FS

type Themes struct {
	themes *template.Template
	l      *slog.Logger
}

func New(logger *slog.Logger) (*Themes, error) {
	themes := &Themes{
		themes: template.New("root"),
		l:      logger,
	}

	err := themes.ParseTemplatesFS(embeddedThemesFS)
	if err != nil {
		// Should never happen
		logger.Error("could not parse embedded templates", slog.Any("reason", err))
		return nil, err
	}

	return themes, nil
}

func NewWithCustomThemes(custom fs.FS, logger *slog.Logger) (*Themes, error) {
	themes := &Themes{
		themes: template.New("root"),
		l:      logger,
	}

	err := themes.ParseTemplatesFS(embeddedThemesFS)
	if err != nil {
		// Should never happen
		logger.Error("could not parse embedded templates", slog.Any("reason", err))
		return nil, err
	}

	err = themes.ParseTemplatesFS(custom)
	if err != nil {
		logger.Error("could not parse custom templates", slog.Any("reason", err))
		return nil, err
	}

	return themes, nil
}
