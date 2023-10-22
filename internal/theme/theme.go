package theme

import (
	"embed"
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

func NewWithCustomThemes(custom fs.FS) *Themes {
	themes := &Themes{
		themes: template.New("root"),
	}

	load(themes.themes, embeddedThemesFS)
	load(themes.themes, custom)

	return themes
}

func load(t *template.Template, fs fs.FS) {
	if t != nil {
		t.ParseFS(fs, "*/*.html")
		t.ParseFS(fs, "*.html")
	}
}

type InstanceInfo struct {
	Name   string
	Status string
	Error  error
}

type ThemeOptions struct {
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

func (t *Themes) Execute(writer io.Writer, name string, opts ThemeOptions) error {

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
