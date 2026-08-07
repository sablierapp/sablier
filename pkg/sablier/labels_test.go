package sablier

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/sablierapp/sablier/internal/labeldoc"
)

// TestLabelsSingleSourceOfTruth verifies two things:
//  1. every label constant in labels.go has a complete, decodable doc comment;
//  2. no non-test code reads a "sablier.*" label (labels["sablier.x"]) that is
//     not declared in labels.go.
//
// Together these make pkg/sablier/labels.go the single source of truth: a label
// used anywhere in the code must be declared and documented here, and therefore
// appears in the generated Label reference page.
func TestLabelsSingleSourceOfTruth(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file path")
	}
	sablierDir := filepath.Dir(thisFile)                 // .../pkg/sablier
	labelsFile := filepath.Join(sablierDir, "labels.go") // .../pkg/sablier/labels.go
	pkgDir := filepath.Dir(sablierDir)                   // .../pkg

	labels, err := labeldoc.Parse(labelsFile)
	if err != nil {
		t.Fatalf("parse labels.go: %v", err)
	}
	if len(labels) == 0 {
		t.Fatal("no labels parsed from labels.go")
	}

	valid := make(map[string]bool, len(labels))
	for _, l := range labels {
		valid[l.Name] = true
		if l.Description == "" {
			t.Errorf("label %s (%s) has no description", l.Const, l.Name)
		}
		if l.Type == "" {
			t.Errorf("label %s (%s) has no Type", l.Const, l.Name)
		}
	}

	// Labels are always read by indexing a labels/annotations map, e.g.
	// labels["sablier.idle.cpu"]. The bracket form targets label reads precisely
	// and ignores same-prefixed identifiers such as tracing span names.
	re := regexp.MustCompile(`\["(sablier\.[A-Za-z0-9._-]+)"\]`)
	err = filepath.WalkDir(pkgDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range re.FindAllStringSubmatch(string(b), -1) {
			if !valid[m[1]] {
				t.Errorf("%s: label literal %q is not declared in pkg/sablier/labels.go; add it there (or use a Label* constant)", path, m[1])
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
