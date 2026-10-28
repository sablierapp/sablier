package bpf

import (
	"errors"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

// TestSablierDoesNotLinkRlimit guards "nothing eBPF-related is loaded" with
// --activity.enabled=false: the init function of github.com/cilium/ebpf/rlimit
// lowers RLIMIT_MEMLOCK and calls bpf(BPF_MAP_CREATE) in every process that
// links it, so a seccomp policy that kills on bpf() would kill Sablier at
// startup.
func TestSablierDoesNotLinkRlimit(t *testing.T) {
	gobin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("the go tool is not available")
	}
	for _, arch := range []string{"amd64", "arm64"} {
		t.Run(arch, func(t *testing.T) {
			cmd := exec.Command(gobin, "list", "-deps", "github.com/sablierapp/sablier/cmd/sablier")
			cmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH="+arch, "CGO_ENABLED=0")
			out, err := cmd.Output()
			var stderr string
			if exitErr := (*exec.ExitError)(nil); errors.As(err, &exitErr) {
				stderr = string(exitErr.Stderr)
			}
			assert.NilError(t, err, stderr)

			deps := strings.Fields(string(out))
			assert.Assert(t, slices.Contains(deps, "github.com/cilium/ebpf"), "the tracker is not linked: the guard checks nothing")
			assert.Assert(t, !slices.Contains(deps, "github.com/cilium/ebpf/rlimit"),
				"cmd/sablier links github.com/cilium/ebpf/rlimit, whose init function calls bpf() in every process")
		})
	}
}
