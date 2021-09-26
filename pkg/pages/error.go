package pages

import (
	"bytes"
	"html/template"
  _ "embed"
)

//go:embed error.html
var errorPage string

type ErrorData struct {
	name string
	err  string
}

func GetErrorPage(name string, e string) string {
	tpl, err := template.New("error").Parse(errorPage)
	if err != nil {
		return err.Error()
	}
	b := bytes.Buffer{}
	tpl.Execute(&b, ErrorData{
		name: name,
		err:  e,
	})
	return b.String()
}
