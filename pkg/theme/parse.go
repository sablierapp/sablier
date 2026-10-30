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

// isKubeletDir reports whether the walked entry is one of the "..<timestamp>"
// directories in which Kubernetes keeps the files of a ConfigMap, Secret or
// projected volume. Walking it registered every theme under a name that
// changes with every update of the volume; the files are read through "..data"
// instead (see kubeletVolumeData).
func isKubeletDir(root, filePath string, d fs.DirEntry) bool {
	return d.IsDir() && filePath != root && strings.HasPrefix(d.Name(), "..")
}

// kubeletVolumeData returns the "..data" directory of dir, and whether there is
// one, which makes dir the root of a Kubernetes ConfigMap, Secret or projected
// volume. Such a volume links every top-level entry through "..data", so a key
// mapped into a sub-directory with items[].path is reached only through a link
// to a directory, which fs.WalkDir does not follow. "..data" holds every key at
// its path.
func kubeletVolumeData(f fs.FS, dir string) (string, bool) {
	data := path.Join(dir, "..data")
	info, err := fs.Stat(f, data)
	return data, err == nil && info.IsDir()
}

// relativePath returns filePath, as fs.WalkDir reports it for a walk from root,
// relative to root.
func relativePath(root, filePath string) string {
	if root == "." {
		return filePath
	}
	if filePath == root {
		return "."
	}
	return strings.TrimPrefix(filePath, root+"/")
}

// isLinkToFile reports whether the theme symlink at filePath resolves to a
// regular file. A link that leaves the themes directory, or that points to a
// directory, would otherwise fail in bundleHTML and abort loading every custom
// theme, so it is skipped with a warning instead.
func (t *Themes) isLinkToFile(f fs.FS, filePath string) bool {
	info, err := fs.Stat(f, filePath)
	if err != nil {
		t.l.Warn("skipping theme", slog.String("path", filePath), slog.Any("reason", err))
		return false
	}
	if !info.Mode().IsRegular() {
		t.l.Warn("skipping theme", slog.String("path", filePath), slog.String("reason", "symlink does not point to a regular file"))
		return false
	}
	return true
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
// in different sub-directories silently overwrite each other. A theme in a
// Kubernetes volume is named after its path in the volume (see
// kubeletVolumeData).
func (t *Themes) ParseAndBundleTemplatesFS(f fs.FS) error {
	return t.parseAndBundleDir(f, ".", ".")
}

// parseAndBundleDir registers the themes below root, naming each after its
// path relative to root, joined to prefix.
func (t *Themes) parseAndBundleDir(f fs.FS, root, prefix string) error {
	return fs.WalkDir(f, root, func(filePath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if isKubeletDir(root, filePath, d) {
			return fs.SkipDir
		}
		if d.IsDir() {
			if data, ok := kubeletVolumeData(f, filePath); ok {
				return t.parseAndBundleDir(f, data, path.Join(prefix, relativePath(root, filePath)))
			}
			return nil
		}
		if !isThemeFile(filePath, d) {
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			// The top-level keys of a volume are read through "..data" already.
			if _, ok := kubeletVolumeData(f, path.Dir(filePath)); ok {
				return nil
			}
			if !t.isLinkToFile(f, filePath) {
				return nil
			}
		}
		t.l.Info("theme found", slog.String("path", filePath))
		content, err := bundleHTML(f, filePath)
		if err != nil {
			t.l.Info("cannot bundle theme", slog.String("path", filePath), slog.Any("reason", err))
			return err
		}
		// Relative paths are unique, so the only name a custom theme can already
		// occupy is that of an embedded theme, which it deliberately replaces.
		name := path.Join(prefix, relativePath(root, filePath))
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
