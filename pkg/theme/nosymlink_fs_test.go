package theme

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNoSymlinkFS_PassesThroughRegularFiles(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "style.css"), []byte(`body{}`), 0o644))

	fsys := noSymlinkFS{FS: os.DirFS(dir), root: dir}
	data, err := fs.ReadFile(fsys, "style.css")
	require.NoError(t, err)
	assert.Equal(t, []byte(`body{}`), data)
}

func TestNoSymlinkFS_BlocksSymlinkedFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "real.txt")
	link := filepath.Join(dir, "link.css")
	require.NoError(t, os.WriteFile(target, []byte(`secret`), 0o644))

	if err := os.Symlink(target, link); err != nil {
		t.Skipf("cannot create symlinks on this platform: %v", err)
	}

	fsys := noSymlinkFS{FS: os.DirFS(dir), root: dir}
	_, err := fs.ReadFile(fsys, "link.css")
	require.Error(t, err, "expected an error when opening a symlinked file")
	assert.Contains(t, err.Error(), "symlinks are not allowed")
}

func TestNoSymlinkFS_BlocksSymlinkedFileInSubdir(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "css")
	require.NoError(t, os.MkdirAll(subdir, 0o755))

	target := filepath.Join(dir, "secrets.txt")
	link := filepath.Join(subdir, "style.css")
	require.NoError(t, os.WriteFile(target, []byte(`very secret`), 0o644))

	if err := os.Symlink(target, link); err != nil {
		t.Skipf("cannot create symlinks on this platform: %v", err)
	}

	fsys := noSymlinkFS{FS: os.DirFS(dir), root: dir}
	_, err := fs.ReadFile(fsys, "css/style.css")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "symlinks are not allowed")
}

func TestNoSymlinkFS_ReadDirFiltersSymlinks(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "real.css"), []byte(``), 0o644))

	target := filepath.Join(dir, "real.css")
	link := filepath.Join(dir, "linked.css")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("cannot create symlinks on this platform: %v", err)
	}

	fsys := noSymlinkFS{FS: os.DirFS(dir), root: dir}
	entries, err := fsys.ReadDir(".")
	require.NoError(t, err)

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	assert.Contains(t, names, "real.css", "regular file should appear in listing")
	assert.NotContains(t, names, "linked.css", "symlink should be filtered out of listing")
}

func TestNoSymlinkFS_MapFSHasNoSymlinks(t *testing.T) {
	// fstest.MapFS has no symlinks; noSymlinkFS should pass through all files.
	inner := fstest.MapFS{
		"a.css": {Data: []byte(`.a{}`)},
	}
	// noSymlinkFS with an empty root falls back to letting the inner FS handle Lstat.
	fsys := noSymlinkFS{FS: inner, root: t.TempDir()}
	// The file doesn't exist on the real OS filesystem, so Lstat will fail and
	// noSymlinkFS delegates to the inner FS — which succeeds.
	data, err := fs.ReadFile(fsys, "a.css")
	require.NoError(t, err)
	assert.Equal(t, []byte(`.a{}`), data)
}
