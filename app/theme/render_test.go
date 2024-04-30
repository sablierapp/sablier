package theme_test

import (
	"bytes"
	"fmt"
	"os"
	"testing"
	"testing/fstest"
	"time"

	"github.com/acouvreur/sablier/app/theme"
	"github.com/acouvreur/sablier/version"
)

var (
	StartingInstanceInfo = theme.Instance{
		Name:            "starting-instance",
		Status:          "instance is starting...",
		Error:           nil,
		CurrentReplicas: 0,
		DesiredReplicas: 1,
	}
	StartedInstanceInfo = theme.Instance{
		Name:            "started-instance",
		Status:          "instance is started.",
		Error:           nil,
		CurrentReplicas: 1,
		DesiredReplicas: 1,
	}
	ErrorInstanceInfo = theme.Instance{
		Name:            "error-instance",
		Error:           fmt.Errorf("instance does not exist"),
		CurrentReplicas: 0,
		DesiredReplicas: 1,
	}
)

func TestThemes_Render(t *testing.T) {
	const customTheme = `
<!DOCTYPE html>
<html lang="en">
<head>
	<meta http-equiv="refresh" content="{{ .RefreshFrequency }}" />
</head>
<body>
	Starting</span> {{ .DisplayName }}
	Your instance(s) will stop after {{ .SessionDuration }} of inactivity
		
	<table>
		{{- range $i, $instance := .InstanceStates }}
		<tr>
			<td>{{ $instance.Name }}</td>
			{{- if $instance.Error }}
			<td>{{ $instance.Error }}</td>
			{{- else }}
			<td>{{ $instance.Status }} ({{ $instance.CurrentReplicas }}/{{ $instance.DesiredReplicas }})</td>
			{{- end}}
		</tr>
		{{ end -}}
	</table>
	Sablier version {{ .Version }}
</body>
</html>
`
	version.Version = "1.0.0"
	themes, err := theme.NewWithCustomThemes(fstest.MapFS{
		"inner/custom-theme.html": &fstest.MapFile{Data: []byte(customTheme)},
	})
	if err != nil {
		t.Error(err)
		return
	}

	instances := []theme.Instance{
		StartingInstanceInfo,
		StartedInstanceInfo,
		ErrorInstanceInfo,
	}
	options := theme.Options{
		DisplayName:      "Test",
		InstanceStates:   instances,
		SessionDuration:  10 * time.Minute,
		RefreshFrequency: 5 * time.Second,
	}
	type args struct {
		name string
		opts theme.Options
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "Load ghost theme",
			args: args{
				name: "ghost",
				opts: options,
			},
			wantErr: false,
		},
		{
			name: "Load hacker-terminal theme",
			args: args{
				name: "hacker-terminal",
				opts: options,
			},
			wantErr: false,
		},
		{
			name: "Load matrix theme",
			args: args{
				name: "matrix",
				opts: options,
			},
			wantErr: false,
		},
		{
			name: "Load shuffle theme",
			args: args{
				name: "shuffle",
				opts: options,
			},
			wantErr: false,
		},
		{
			name: "Load non existent theme",
			args: args{
				name: "non-existent",
				opts: options,
			},
			wantErr: true,
		},
		{
			name: "Load custom theme",
			args: args{
				name: "custom-theme",
				opts: options,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writer := &bytes.Buffer{}
			if err := themes.Render(tt.args.name, tt.args.opts, writer); (err != nil) != tt.wantErr {
				t.Errorf("Themes.Render() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func ExampleThemes_Render() {
	const customTheme = `
<html lang="en">
	<head>
		<meta http-equiv="refresh" content="{{ .RefreshFrequency }}" />
	</head>
	<body>
		Starting {{ .DisplayName }}
		Your instances will stop after {{ .SessionDuration }} of inactivity
		<table>
			{{- range $i, $instance := .InstanceStates }}
			<tr>
				<td>{{ $instance.Name }}</td>
				{{- if $instance.Error }}
				<td>{{ $instance.Error }}</td>
				{{- else }}
				<td>{{ $instance.Status }} ({{ $instance.CurrentReplicas }}/{{ $instance.DesiredReplicas }})</td>
				{{- end}}
			</tr>
			{{- end }}
		</table>
		Sablier version {{ .Version }}
	</body>
</html>
`
	version.Version = "1.0.0"
	themes, err := theme.NewWithCustomThemes(fstest.MapFS{
		"inner/custom-theme.html": &fstest.MapFile{Data: []byte(customTheme)},
	})
	if err != nil {
		panic(err)
	}
	instances := []theme.Instance{
		StartingInstanceInfo,
		StartedInstanceInfo,
		ErrorInstanceInfo,
	}

	err = themes.Render("custom-theme", theme.Options{
		DisplayName:      "Test",
		InstanceStates:   instances,
		ShowDetails:      true,
		SessionDuration:  10 * time.Minute,
		RefreshFrequency: 5 * time.Second,
	}, os.Stdout)

	if err != nil {
		panic(err)
	}

	// Output:
	//<html lang="en">
	//	<head>
	//		<meta http-equiv="refresh" content="5" />
	//	</head>
	//	<body>
	//		Starting Test
	//		Your instances will stop after 10 minutes of inactivity
	//		<table>
	//			<tr>
	//				<td>starting-instance</td>
	//				<td>instance is starting... (0/1)</td>
	//			</tr>
	//			<tr>
	//				<td>started-instance</td>
	//				<td>instance is started. (1/1)</td>
	//			</tr>
	//			<tr>
	//				<td>error-instance</td>
	//				<td>instance does not exist</td>
	//			</tr>
	//		</table>
	//		Sablier version 1.0.0
	//	</body>
	//</html>
}
