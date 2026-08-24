package theme

import (
	"io/fs"
	"log/slog"
	"path"
	"strings"
)

// isThemeFile reports whether the walked entry is a theme template: a regular
// file whose extension is exactly ".html".
//
// The check used to be strings.Contains(filePath, ".html"), which also matched
// directories such as "partials.html/" and files such as "theme.html.bak". A
// matching directory was then handed to bundleHTML, whose read failed and
// aborted the whole walk, so a single oddly named folder silently cost the user
// every one of their custom themes.
func isThemeFile(filePath string, d fs.DirEntry) bool {
	return d != nil && !d.IsDir() && path.Ext(filePath) == ".html"
}

func (t *Themes) ParseTemplatesFS(f fs.FS) error {
	return fs.WalkDir(f, ".", func(filePath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !isThemeFile(filePath, d) {
			return nil
		}
		t.l.Info("theme found", slog.String("path", filePath))
		if _, err = t.themes.ParseFS(f, filePath); err != nil {
			t.l.Info("cannot add theme", slog.String("path", filePath), slog.Any("reason", err))
			return err
		}

		t.l.Info("successfully added theme", slog.String("path", filePath))
		return nil
	})
}

// ParseAndBundleTemplatesFS walks f and registers every .html file as a named
// template, inlining relative CSS, JS, and image assets so that the resulting
// template is fully self-contained (see bundleHTML for details).
//
// A theme is named after its path relative to the themes directory, without the
// ".html" extension, so that "special/secret.html" is requested as
// "special/secret". Naming themes after their base name instead let two files
// in different sub-directories silently overwrite each other.
func (t *Themes) ParseAndBundleTemplatesFS(f fs.FS) error {
	return fs.WalkDir(f, ".", func(filePath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !isThemeFile(filePath, d) {
			return nil
		}
		t.l.Info("theme found", slog.String("path", filePath))
		content, err := bundleHTML(f, filePath)
		if err != nil {
			t.l.Info("cannot bundle theme", slog.String("path", filePath), slog.Any("reason", err))
			return err
		}
		// Relative paths are unique, so the only name a custom theme can already
		// occupy is that of an embedded theme, which it deliberately replaces.
		name := path.Clean(filePath)
		if t.themes.Lookup(name) != nil {
			t.l.Info("theme overrides an embedded theme", slog.String("path", filePath), slog.String("name", strings.TrimSuffix(name, ".html")))
		}
		if _, err = t.themes.New(name).Parse(content); err != nil {
			t.l.Info("cannot add theme", slog.String("path", filePath), slog.Any("reason", err))
			return err
		}
		t.l.Info("successfully added theme", slog.String("path", filePath), slog.String("name", strings.TrimSuffix(name, ".html")))
		return nil
	})
}
