package theme

import (
	"fmt"
	"io"

	"github.com/sablierapp/sablier/pkg/version"

	"github.com/sablierapp/sablier/pkg/durations"
)

func (t *Themes) Render(name string, opts Options, writer io.Writer) error {
	var instances []Instance

	if opts.ShowDetails {
		instances = opts.InstanceStates
	} else {
		instances = []Instance{}
	}

	// The waiting page refreshes with `<meta http-equiv="refresh" content="N">`, which
	// counts in whole seconds and reads a `0` as "refresh immediately" — a browser keeps
	// reloading. Truncating an interval under a second into that `0` would ask for the
	// fastest refresh a browser can do where the caller asked for a slow one, or for none
	// at all, so the shortest interval that can be served is one second.
	seconds := int64(opts.RefreshFrequency.Seconds())
	if seconds < 1 {
		seconds = 1
	}

	options := TemplateData{
		DisplayName:      opts.DisplayName,
		InstanceStates:   instances,
		SessionDuration:  durations.Humanize(opts.SessionDuration),
		RefreshFrequency: fmt.Sprintf("%d", seconds),
		Version:          version.Version,
	}

	tpl := t.themes.Lookup(fmt.Sprintf("%s.html", name))
	if tpl == nil {
		return ErrThemeNotFound{
			Theme:           name,
			AvailableThemes: t.List(),
		}
	}

	return tpl.Execute(writer, options)
}
