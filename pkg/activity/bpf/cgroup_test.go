package bpf

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
)

func TestCheckCgroup_MissingNamesCgroupHost(t *testing.T) {
	path := filepath.Join(t.TempDir(), "docker", "0123abcd")

	_, err := checkCgroup(path)

	assert.Error(t, err, "cgroup "+path+" not found: run Sablier with cgroup: host on the Docker host")
	assert.Assert(t, errors.Is(err, fs.ErrNotExist), "%v", err)
}

func TestCheckCgroup_Exists(t *testing.T) {
	path := t.TempDir()

	cg, err := checkCgroup(path)

	assert.NilError(t, err)
	assert.Assert(t, cg.IsDir())
}

func TestCgroupReplaced_SameDirectory(t *testing.T) {
	path := t.TempDir()
	cg, err := checkCgroup(path)
	assert.NilError(t, err)

	replaced, err := cgroupReplaced(path, cg)

	assert.NilError(t, err)
	assert.Assert(t, !replaced)
}

// A container's cgroup is removed when it stops.
func TestCgroupReplaced_Missing(t *testing.T) {
	path := t.TempDir()
	cg, err := checkCgroup(path)
	assert.NilError(t, err)
	assert.NilError(t, os.Remove(path))

	replaced, err := cgroupReplaced(path, cg)

	assert.NilError(t, err)
	assert.Assert(t, replaced)
}

// A restarted container gets a new cgroup at the same path. Regular
// filesystems may reuse a removed directory's inode, unlike cgroupfs, so
// compare with another directory that exists at the same time.
func TestCgroupReplaced_OtherDirectory(t *testing.T) {
	cg, err := checkCgroup(t.TempDir())
	assert.NilError(t, err)

	replaced, err := cgroupReplaced(t.TempDir(), cg)

	assert.NilError(t, err)
	assert.Assert(t, replaced)
}
