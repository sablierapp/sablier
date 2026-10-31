package client

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoSwitchInitWithCallTag forbids `switch v := x(); v.Kind() {`: Yaegi,
// which Traefik runs plugins with, matches no case of a switch with an init
// statement and a call as its tag, without any error. No linter reports this
// form, and the other Yaegi gaps are forbidden in .golangci.yml.
func TestNoSwitchInitWithCallTag(t *testing.T) {
	sources, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, source := range sources {
		// Plugins only interpret the client, not its tests.
		if strings.HasSuffix(source, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, source, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			if s, ok := n.(*ast.SwitchStmt); ok && s.Init != nil {
				if _, call := s.Tag.(*ast.CallExpr); call {
					t.Errorf("%s: Yaegi matches no case of a switch with an init statement and a call as its tag; assign before the switch", fset.Position(s.Pos()))
				}
			}
			return true
		})
	}
}
