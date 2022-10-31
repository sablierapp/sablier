package pages

import (
	"bytes"
	"fmt"
	"testing"
	"testing/fstest"
	"time"
)

var instanceStates []RenderOptionsInstanceState = []RenderOptionsInstanceState{
	{
		Name:            "nginx",
		CurrentReplicas: 0,
		DesiredReplicas: 4,
		Status:          "starting",
		Error:           nil,
	},
	{
		Name:            "whoami",
		CurrentReplicas: 4,
		DesiredReplicas: 4,
		Status:          "started",
		Error:           nil,
	},
	{
		Name:            "devil",
		CurrentReplicas: 0,
		DesiredReplicas: 4,
		Status:          "error",
		Error:           fmt.Errorf("devil service does not exist"),
	},
}

func TestRender(t *testing.T) {
	type args struct {
		options RenderOptions
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "Load ghost theme",
			args: args{
				options: RenderOptions{
					DisplayName:      "Test",
					InstanceStates:   instanceStates,
					Theme:            "ghost",
					SessionDuration:  10 * time.Minute,
					RefreshFrequency: 5 * time.Second,
					CustomThemes:     nil,
					Version:          "v0.0.0",
				},
			},
			wantErr: false,
		},
		{
			name: "Load hacker-terminal theme",
			args: args{
				options: RenderOptions{
					DisplayName:      "Test",
					InstanceStates:   instanceStates,
					Theme:            "hacker-terminal",
					SessionDuration:  10 * time.Minute,
					RefreshFrequency: 5 * time.Second,
					CustomThemes:     nil,
					Version:          "v0.0.0",
				},
			},
			wantErr: false,
		},
		{
			name: "Load matrix theme",
			args: args{
				options: RenderOptions{
					DisplayName:      "Test",
					InstanceStates:   instanceStates,
					Theme:            "matrix",
					SessionDuration:  10 * time.Minute,
					RefreshFrequency: 5 * time.Second,
					CustomThemes:     nil,
					Version:          "v0.0.0",
				},
			},
			wantErr: false,
		},
		{
			name: "Load shiffle theme",
			args: args{
				options: RenderOptions{
					DisplayName:      "Test",
					InstanceStates:   instanceStates,
					Theme:            "shuffle",
					SessionDuration:  10 * time.Minute,
					RefreshFrequency: 5 * time.Second,
					CustomThemes:     nil,
					Version:          "v0.0.0",
				},
			},
			wantErr: false,
		},
		{
			name: "Load non existant theme",
			args: args{
				options: RenderOptions{
					DisplayName:      "Test",
					InstanceStates:   instanceStates,
					Theme:            "nonexistant",
					SessionDuration:  10 * time.Minute,
					RefreshFrequency: 5 * time.Second,
					CustomThemes:     nil,
					Version:          "v0.0.0",
				},
			},
			wantErr: true,
		},
		{
			name: "Load custom theme",
			args: args{
				options: RenderOptions{
					DisplayName:      "Test",
					InstanceStates:   instanceStates,
					Theme:            "dc-comics",
					SessionDuration:  10 * time.Minute,
					RefreshFrequency: 5 * time.Second,
					CustomThemes: fstest.MapFS{
						"marvel.html":    {Data: []byte("{{ .DisplayName }}")},
						"dc-comics.html": {Data: []byte("batman")},
					},
					Version: "v0.0.0",
				},
			},
			wantErr: false,
		},
		{
			name: "Load non existant custom theme",
			args: args{
				options: RenderOptions{
					DisplayName:      "Test",
					InstanceStates:   instanceStates,
					Theme:            "nonexistant",
					SessionDuration:  10 * time.Minute,
					RefreshFrequency: 5 * time.Second,
					CustomThemes: fstest.MapFS{
						"marvel.html":    {Data: []byte("thor")},
						"dc-comics.html": {Data: []byte("batman")},
					},
					Version: "v0.0.0",
				},
			},
			wantErr: true,
		},
		{
			name: "Load embedded theme with custom theme provided",
			args: args{
				options: RenderOptions{
					DisplayName:      "Test",
					InstanceStates:   instanceStates,
					Theme:            "hacker-terminal",
					SessionDuration:  10 * time.Minute,
					RefreshFrequency: 5 * time.Second,
					CustomThemes: fstest.MapFS{
						"marvel.html":    {Data: []byte("thor")},
						"dc-comics.html": {Data: []byte("batman")},
					},
					Version: "v0.0.0",
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writer := &bytes.Buffer{}
			if err := Render(tt.args.options, writer); (err != nil) != tt.wantErr {
				t.Errorf("Render() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}
