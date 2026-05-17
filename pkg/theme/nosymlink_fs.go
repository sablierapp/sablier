package theme

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// noSymlinkFS wraps an fs.FS rooted at a directory and refuses to open any
// path that resolves to a symbolic link on the underlying OS filesystem.
//
// This prevents a theme author (or an attacker who gains write access to the
// themes directory) from creating a symlink such as
//
//	themes/css/secrets.css -> /etc/passwd
//
// and having Sablier inline the target file's contents into the served HTML.
//
// The wrapper is used by NewWithCustomThemesFromPath, which is the only
// production code path that reads from the OS filesystem.
type noSymlinkFS struct {
	fs.FS
	root string // absolute path of the theme directory
}

// Open implements fs.FS. It calls os.Lstat on the real path before delegating
// to the inner FS so that symlinks are caught regardless of the underlying
// fs.FS implementation.
func (n noSymlinkFS) Open(name string) (fs.File, error) {
	real := filepath.Join(n.root, filepath.FromSlash(name))
	info, err := os.Lstat(real)
	if err != nil {
		// File may not exist; let the inner FS produce the canonical error.
		return n.FS.Open(name)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, &fs.PathError{
			Op:   "open",
			Path: name,
			Err:  errors.New("symlinks are not allowed in theme directories"),
		}
	}
	return n.FS.Open(name)
}

// ReadDir implements fs.ReadDirFS. It delegates to the inner FS and removes
// any symlink entries from the listing so that fs.WalkDir never attempts to
// visit them; the Open guard above acts as a second line of defence.
func (n noSymlinkFS) ReadDir(name string) ([]fs.DirEntry, error) {
	rfs, ok := n.FS.(fs.ReadDirFS)
	if !ok {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: errors.ErrUnsupported}
	}
	entries, err := rfs.ReadDir(name)
	if err != nil {
		return nil, err
	}
	filtered := make([]fs.DirEntry, 0, len(entries))
	for _, e := range entries {
		if e.Type()&fs.ModeSymlink == 0 {
			filtered = append(filtered, e)
		}
	}
	return filtered, nil
}
