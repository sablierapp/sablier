package theme

import (
	"io/fs"
	"log/slog"
	"path"
	"strings"
)

func (t *Themes) ParseTemplatesFS(f fs.FS) error {
	err := fs.WalkDir(f, ".", func(filePath string, d fs.DirEntry, err error) error {
		if strings.Contains(filePath, ".html") {
			t.l.Info("theme found", slog.String("path", filePath))
			_, err = t.themes.ParseFS(f, filePath)
			if err != nil {
				t.l.Info("cannot add theme", slog.String("path", filePath), slog.Any("reason", err))
				return err
			}

			t.l.Info("successfully added theme", slog.String("path", filePath))
		}
		return err
	})

	return err
}

// ParseAndBundleTemplatesFS walks f and registers every .html file as a named
// template, inlining relative CSS, JS, and image assets so that the resulting
// template is fully self-contained (see bundleHTML for details).
func (t *Themes) ParseAndBundleTemplatesFS(f fs.FS) error {
	return fs.WalkDir(f, ".", func(filePath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !strings.Contains(filePath, ".html") {
			return nil
		}
		t.l.Info("theme found", slog.String("path", filePath))
		content, err := bundleHTML(f, filePath)
		if err != nil {
			t.l.Info("cannot bundle theme", slog.String("path", filePath), slog.Any("reason", err))
			return err
		}
		name := path.Base(filePath)
		if _, err = t.themes.New(name).Parse(content); err != nil {
			t.l.Info("cannot add theme", slog.String("path", filePath), slog.Any("reason", err))
			return err
		}
		t.l.Info("successfully added theme", slog.String("path", filePath))
		return nil
	})
}
