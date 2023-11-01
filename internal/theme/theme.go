package theme

import (
	"embed"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"time"

	"github.com/acouvreur/sablier/pkg/durations"
	"github.com/acouvreur/sablier/version"
)

// List of built-it themes
//
//go:embed embedded/*.html
var embeddedThemesFS embed.FS

type Themes struct {
	themes *template.Template
}

func New() *Themes {
	themes := &Themes{
		themes: template.New("root"),
	}

	load(themes.themes, embeddedThemesFS)

	return themes
}

func NewWithCustomThemes(custom fs.FS) (*Themes, error) {
	themes := &Themes{
		themes: template.New("root"),
	}

	err := load(themes.themes, embeddedThemesFS)
	if err != nil {
		// Should never happen
		return nil, err
	}

	err = load(themes.themes, custom)
	if err != nil {
		return nil, err
	}

	return themes, nil
}

func load(t *template.Template, fs fs.FS) error {
	if t == nil {
		return errors.New("template is nil")
	}

	_, err := t.ParseFS(fs, "*/*.html")
	if err != nil {
		return err
	}
	_, err = t.ParseFS(fs, "*.html")
	if err != nil {
		return err
	}

	return nil
}

type InstanceInfo struct {
	Name   string
	Status string
	Error  error
}

type Options struct {
	Title            string
	DisplayName      string
	ShowDetails      bool
	Instances        []InstanceInfo
	SessionDuration  time.Duration
	RefreshFrequency time.Duration
}

type templateData struct {
	Title                   string
	DisplayName             string
	Instances               []InstanceInfo
	SessionDuration         string
	RefreshFrequencySeconds string
	Version                 string
}

func (t *Themes) Execute(writer io.Writer, name string, opts Options) error {
	var instances []InstanceInfo

	if opts.ShowDetails {
		instances = opts.Instances
	} else {
		instances = []InstanceInfo{}
	}

	options := templateData{
		Title:                   opts.Title,
		DisplayName:             opts.DisplayName,
		Instances:               instances,
		SessionDuration:         durations.Humanize(opts.SessionDuration),
		RefreshFrequencySeconds: fmt.Sprintf("%d", int64(opts.RefreshFrequency.Seconds())),
		Version:                 version.Info(),
	}

	tpl := t.themes.Lookup(fmt.Sprintf("%s.html", name))
	if tpl == nil {
		return fmt.Errorf("theme %s does not exist", name)
	}

	return tpl.Execute(writer, options)
}
