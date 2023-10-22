package theme_test

import (
	"bytes"
	"fmt"
	"os"
	"testing"
	"testing/fstest"
	"time"

	"github.com/acouvreur/sablier/internal/theme"
)

var (
	StartingInstanceInfo theme.InstanceInfo = theme.InstanceInfo{
		Name:   "starting-instance",
		Status: "instance is starting...",
		Error:  nil,
	}
	StartedInstanceInfo theme.InstanceInfo = theme.InstanceInfo{
		Name:   "started-instance",
		Status: "instance is started.",
		Error:  nil,
	}
	ErrorInstanceInfo theme.InstanceInfo = theme.InstanceInfo{
		Name:  "error-instance",
		Error: fmt.Errorf("instance does not exist"),
	}
)

func TestThemes_Execute(t *testing.T) {
	const customTheme = `
<!DOCTYPE html>
<html lang="en">
<head>
	<title>{{ .Title }}</title>
	<meta http-equiv="refresh" content="{{ .RefreshFrequencySeconds }}" />
</head>
<body>
	Starting</span> {{ .DisplayName }}
	Your instance(s) will stop after {{ .SessionDuration }} of inactivity
		
	<table>
		{{- range $i, $instance := .Instances }}
		<tr>
			<td>{{ $instance.Name }}</td>
			{{- if $instance.Error }}
			<td>{{ $instance.Error }}</td>
			{{- else }}
			<td>{{ $instance.Status }}</td>
			{{- end}}
		</tr>
		{{ end -}}
	</table>
</body>
</html>
`
	themes := theme.NewWithCustomThemes(fstest.MapFS{
		"inner/custom-theme.html": &fstest.MapFile{Data: []byte(customTheme)},
	})
	instances := []theme.InstanceInfo{
		StartingInstanceInfo,
		StartedInstanceInfo,
		ErrorInstanceInfo,
	}
	type args struct {
		name string
		opts theme.ThemeOptions
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
				opts: theme.ThemeOptions{
					DisplayName:      "Test",
					Instances:        instances,
					SessionDuration:  10 * time.Minute,
					RefreshFrequency: 5 * time.Second,
				},
			},
			wantErr: false,
		},
		{
			name: "Load hacker-terminal theme",
			args: args{
				name: "hacker-terminal",
				opts: theme.ThemeOptions{
					DisplayName:      "Test",
					Instances:        instances,
					SessionDuration:  10 * time.Minute,
					RefreshFrequency: 5 * time.Second,
				},
			},
			wantErr: false,
		},
		{
			name: "Load matrix theme",
			args: args{
				name: "matrix",
				opts: theme.ThemeOptions{
					DisplayName:      "Test",
					Instances:        instances,
					SessionDuration:  10 * time.Minute,
					RefreshFrequency: 5 * time.Second,
				},
			},
			wantErr: false,
		},
		{
			name: "Load shuffle theme",
			args: args{
				name: "shuffle",
				opts: theme.ThemeOptions{
					DisplayName:      "Test",
					Instances:        instances,
					SessionDuration:  10 * time.Minute,
					RefreshFrequency: 5 * time.Second,
				},
			},
			wantErr: false,
		},
		{
			name: "Load non existent theme",
			args: args{
				name: "non-existent",
				opts: theme.ThemeOptions{
					DisplayName:      "Test",
					Instances:        instances,
					SessionDuration:  10 * time.Minute,
					RefreshFrequency: 5 * time.Second,
				},
			},
			wantErr: true,
		},
		{
			name: "Load custom theme",
			args: args{
				name: "custom-theme",
				opts: theme.ThemeOptions{
					DisplayName:      "Test",
					Instances:        instances,
					SessionDuration:  10 * time.Minute,
					RefreshFrequency: 5 * time.Second,
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writer := &bytes.Buffer{}
			if err := themes.Execute(writer, tt.args.name, tt.args.opts); (err != nil) != tt.wantErr {
				t.Errorf("Themes.Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func ExampleThemes_Execute() {
	const customTheme = `
<html lang="en">
	<head>
		<title>{{ .Title }}</title>
		<meta http-equiv="refresh" content="{{ .RefreshFrequencySeconds }}" />
	</head>
	<body>
		Starting {{ .DisplayName }}
		Your instances will stop after {{ .SessionDuration }} of inactivity
		<table>
			{{- range $i, $instance := .Instances }}
			<tr>
				<td>{{ $instance.Name }}</td>
				{{- if $instance.Error }}
				<td>{{ $instance.Error }}</td>
				{{- else }}
				<td>{{ $instance.Status }}</td>
				{{- end}}
			</tr>
			{{- end }}
		</table>
	</body>
</html>
`
	themes := theme.NewWithCustomThemes(fstest.MapFS{
		"inner/custom-theme.html": &fstest.MapFile{Data: []byte(customTheme)},
	})
	instances := []theme.InstanceInfo{
		StartingInstanceInfo,
		StartedInstanceInfo,
		ErrorInstanceInfo,
	}

	err := themes.Execute(os.Stdout, "custom-theme", theme.ThemeOptions{
		Title:            "My Title",
		DisplayName:      "Test",
		Instances:        instances,
		ShowDetails:      true,
		SessionDuration:  10 * time.Minute,
		RefreshFrequency: 5 * time.Second,
	})

	if err != nil {
		panic(err)
	}

	// Output:
	//<html lang="en">
	//	<head>
	//		<title>My Title</title>
	//		<meta http-equiv="refresh" content="5" />
	//	</head>
	//	<body>
	//		Starting Test
	//		Your instances will stop after 10 minutes of inactivity
	//		<table>
	//			<tr>
	//				<td>starting-instance</td>
	//				<td>instance is starting...</td>
	//			</tr>
	//			<tr>
	//				<td>started-instance</td>
	//				<td>instance is started.</td>
	//			</tr>
	//			<tr>
	//				<td>error-instance</td>
	//				<td>instance does not exist</td>
	//			</tr>
	//		</table>
	//	</body>
	//</html>
}
