// Package labeldoc parses the structured doc comments on Sablier's label
// constants (pkg/sablier/labels.go) into a form the documentation generator
// (cmd/labelsgen) and tests can consume.
//
// The doc comment convention for each constant is a description followed by
// lowercase "key: value" meta lines:
//
//	// LabelFoo <description, one or more sentences>
//	//
//	// type: <value format>
//	// default: <default value>       (optional)
//	// example: "<example value>"
//	// required: true                 (optional)
//	// since: <version>                (optional)
//	// feature: /features/...          (optional)
//	// providers: <provider caveats>   (optional, may wrap over several lines)
//	LabelFoo = "sablier.foo"
//
// Everything before the first recognised meta line is the description; the
// leading constant identifier (and a leading "is"/"are") is stripped so the
// text reads as a standalone sentence.
package labeldoc

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
	"unicode"
)

// Label is a parsed label constant.
type Label struct {
	Const       string // constant identifier, e.g. "LabelIdleCPU"
	Name        string // label key, e.g. "sablier.idle.cpu"
	Description string
	Type        string
	Default     string
	Example     string
	Required    bool
	Since       string
	Feature     string
	Providers   string
}

// fieldKeys are the recognised lowercase "key:" meta lines in a label doc comment.
var fieldKeys = map[string]bool{
	"type": true, "default": true, "example": true,
	"required": true, "since": true, "feature": true, "providers": true,
}

// Parse reads srcPath (a Go source file) and returns every "sablier.*" string
// constant it declares, in source order, with its doc comment decoded.
func Parse(srcPath string) ([]Label, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, srcPath, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	var labels []Label
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.CONST {
			continue
		}
		for _, spec := range gd.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok || len(vs.Names) != 1 || len(vs.Values) != 1 {
				continue
			}
			lit, ok := vs.Values[0].(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				continue
			}
			value, err := strconv.Unquote(lit.Value)
			if err != nil || !strings.HasPrefix(value, "sablier.") {
				continue
			}
			l := Label{Const: vs.Names[0].Name, Name: value}
			decodeDoc(&l, vs.Doc.Text())
			labels = append(labels, l)
		}
	}
	return labels, nil
}

func decodeDoc(l *Label, doc string) {
	fields := map[string]string{}
	var desc []string
	curKey := ""

	for _, raw := range strings.Split(doc, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			curKey = ""
			continue
		}
		if key, val, ok := splitField(line); ok {
			fields[key] = val
			curKey = key
			continue
		}
		if curKey != "" { // continuation of a wrapped field value
			fields[curKey] = strings.TrimSpace(fields[curKey] + " " + line)
			continue
		}
		desc = append(desc, line)
	}

	l.Type = fields["type"]
	l.Default = fields["default"]
	l.Example = fields["example"]
	l.Since = fields["since"]
	l.Feature = fields["feature"]
	l.Providers = fields["providers"]
	l.Required = strings.EqualFold(fields["required"], "true")
	l.Description = finalizeDescription(l.Const, strings.Join(desc, " "))
}

// splitField returns the key and value of a "Key: value" line, but only for the
// recognised metadata keys, so ordinary description prose is not misread.
func splitField(line string) (key, value string, ok bool) {
	i := strings.Index(line, ": ")
	if i <= 0 {
		return "", "", false
	}
	key = line[:i]
	if !fieldKeys[key] {
		return "", "", false
	}
	return key, strings.TrimSpace(line[i+2:]), true
}

// finalizeDescription strips the leading constant identifier (Go doc comments
// start with the identifier) and a leading "is"/"are", then capitalises, so the
// text reads as a standalone sentence in the generated docs.
func finalizeDescription(constName, s string) string {
	s = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(s), constName))
	for _, p := range []string{"is ", "are "} {
		if strings.HasPrefix(s, p) {
			s = strings.TrimPrefix(s, p)
			break
		}
	}
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}
