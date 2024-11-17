package theme

import (
	"html/template"
	"io/fs"
	"strings"

	log "github.com/sirupsen/logrus"
)

func ParseTemplatesFS(f fs.FS, t *template.Template) error {
	err := fs.WalkDir(f, ".", func(path string, d fs.DirEntry, err error) error {
		if strings.Contains(path, ".html") {
			log.Tracef("found template %s", path)
			_, err = t.ParseFS(f, path)
			if err != nil {
				return err
			}
			log.Tracef("successfully added template %s", path)
		}
		return err
	})

	return err
}
