package theme

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
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

	err = themes.ParseAndBundleTemplatesFS(custom)
	if err != nil {
		logger.Error("could not parse custom templates", slog.Any("reason", err))
		return nil, err
	}

	return themes, nil
}

// NewWithCustomThemesFromPath loads custom themes from a directory on the OS
// filesystem. It is the preferred constructor for production use because it
// opens the directory as an os.Root, which follows a relative symlink only
// while it stays inside the directory, and never an absolute one. A theme
// author (or an attacker who gains write access to the themes directory)
// therefore cannot create a symlink such as
//
//	themes/css -> /etc
//
// and have Sablier inline a file from it into the served HTML. Symlinks that
// stay inside the directory have to be followed, because a Kubernetes ConfigMap
// volume is made of them (see isKubeletDir).
func NewWithCustomThemesFromPath(dirPath string, logger *slog.Logger) (*Themes, error) {
	absPath, err := filepath.Abs(dirPath)
	if err != nil {
		return nil, fmt.Errorf("invalid custom themes path %q: %w", dirPath, err)
	}
	root, err := os.OpenRoot(absPath)
	if err != nil {
		return nil, fmt.Errorf("custom themes path %q is not accessible: %w", absPath, err)
	}
	// Every asset is inlined while the themes are parsed, so nothing reads from
	// the directory afterwards.
	defer root.Close() //nolint:errcheck
	return NewWithCustomThemes(root.FS(), logger)
}
