package theme

import (
	"io/fs"
	"log/slog"
	"strings"
)

func (t *Themes) ParseTemplatesFS(f fs.FS) error {
	err := fs.WalkDir(f, ".", func(path string, d fs.DirEntry, err error) error {
		if strings.Contains(path, ".html") {
			t.l.Info("theme found", slog.String("path", path))
			_, err = t.themes.ParseFS(f, path)
			if err != nil {
				t.l.Info("cannot add theme", slog.String("path", path), slog.Any("reason", err))
				return err
			}

			t.l.Info("successfully added theme", slog.String("path", path))
		}
		return err
	})

	return err
}
