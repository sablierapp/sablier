package theme

import (
	"fmt"
	"io"

	"github.com/acouvreur/sablier/pkg/durations"
	"github.com/acouvreur/sablier/version"
)

func (t *Themes) Render(name string, opts Options, writer io.Writer) error {
	var instances []Instance

	if opts.ShowDetails {
		instances = opts.InstanceStates
	} else {
		instances = []Instance{}
	}

	options := templateOptions{
		DisplayName:      opts.DisplayName,
		InstanceStates:   instances,
		SessionDuration:  durations.Humanize(opts.SessionDuration),
		RefreshFrequency: fmt.Sprintf("%d", int64(opts.RefreshFrequency.Seconds())),
		Version:          version.Version,
	}

	tpl := t.themes.Lookup(fmt.Sprintf("%s.html", name))
	if tpl == nil {
		return fmt.Errorf("theme %s does not exist", name)
	}

	return tpl.Execute(writer, options)
}
