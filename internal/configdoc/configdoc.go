// Package configdoc parses the structured doc comments on the configuration
// struct fields in pkg/config into the metadata the CLI reference generator
// (cmd/docgen) needs, so the documented "since" versions live next to each
// option instead of in a separate hand-maintained map.
//
// Each configuration field documents its flag and introduction version with
// "Key: value" meta lines in its doc comment:
//
//	// Port is the TCP port the Sablier server listens on.
//	// Env: SABLIER_SERVER_PORT
//	// CLI: --server.port
//	// Default: 10000
//	// Since: v1.0.0
//	Port int
//
// SinceByFlag maps the CLI flag name (from the "CLI:" line) to the "Since:"
// value so cmd/docgen can annotate the matching pflag.
package configdoc

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
)

// SinceByFlag parses every .go file in dir and returns a map from flag name
// (without the leading "--") to the "since" value declared in the field's doc
// comment. Fields without both a "CLI:" and a "Since:" line are skipped.
func SinceByFlag(dir string) (map[string]string, error) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	out := map[string]string{}
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			ast.Inspect(file, func(n ast.Node) bool {
				field, ok := n.(*ast.Field)
				if !ok || field.Doc == nil {
					return true
				}
				cli, since := docMeta(field.Doc.Text())
				if cli != "" && since != "" {
					out[strings.TrimPrefix(cli, "--")] = since
				}
				return true
			})
		}
	}
	return out, nil
}

// docMeta extracts the "CLI:" and "Since:" values from a field doc comment.
func docMeta(doc string) (cli, since string) {
	for _, raw := range strings.Split(doc, "\n") {
		line := strings.TrimSpace(raw)
		switch {
		case strings.HasPrefix(line, "CLI:"):
			cli = strings.TrimSpace(strings.TrimPrefix(line, "CLI:"))
		case strings.HasPrefix(line, "Since:"):
			since = strings.TrimSpace(strings.TrimPrefix(line, "Since:"))
		}
	}
	return cli, since
}
