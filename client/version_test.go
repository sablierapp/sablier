package client

import (
	"os"
	"regexp"
	"testing"
)

// TestVersionIsUpdatableByReleasePlease guards the format of version.go:
// Release Please's Go version-file updater only rewrites a line matching this
// pattern (src/updaters/go/version-go.ts), and silently skips the file
// otherwise, e.g. if Version moves into an aligned const block.
func TestVersionIsUpdatableByReleasePlease(t *testing.T) {
	src, err := os.ReadFile("version.go")
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`const Version = "([0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?)"`)
	m := re.FindSubmatch(src)
	if m == nil {
		t.Fatalf("version.go has no line Release Please can update; keep it as: const Version = \"x.y.z\"")
	}
	if string(m[1]) != Version {
		t.Errorf("matched %q, but Version is %q", m[1], Version)
	}
}
