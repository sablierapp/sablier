package bpf

import (
	"errors"
	"io/fs"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
)

func TestCheckCgroup_MissingNamesCgroupHost(t *testing.T) {
	path := filepath.Join(t.TempDir(), "docker", "0123abcd")

	err := checkCgroup(path)

	assert.Error(t, err, "cgroup "+path+" not found: run Sablier with cgroup: host on the Docker host")
	assert.Assert(t, errors.Is(err, fs.ErrNotExist), "%v", err)
}

func TestCheckCgroup_Exists(t *testing.T) {
	assert.NilError(t, checkCgroup(t.TempDir()))
}
