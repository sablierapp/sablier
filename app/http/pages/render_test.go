package pages

import (
	"bytes"
	"fmt"
	"io"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/assert"
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
			name: "Load non existent theme",
			args: args{
				options: RenderOptions{
					DisplayName:      "Test",
					InstanceStates:   instanceStates,
					Theme:            "nonexistent",
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
					AllowedCustomThemes: map[string]bool{
						"marvel":    true,
						"dc-comics": true,
					},
					Version: "v0.0.0",
				},
			},
			wantErr: false,
		},
		{
			name: "Load non existent custom theme",
			args: args{
				options: RenderOptions{
					DisplayName:      "Test",
					InstanceStates:   instanceStates,
					Theme:            "nonexistent",
					SessionDuration:  10 * time.Minute,
					RefreshFrequency: 5 * time.Second,
					CustomThemes: fstest.MapFS{
						"marvel.html":    {Data: []byte("thor")},
						"dc-comics.html": {Data: []byte("batman")},
					},
					AllowedCustomThemes: map[string]bool{
						"marvel":    true,
						"dc-comics": true,
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
					AllowedCustomThemes: map[string]bool{
						"marvel":    true,
						"dc-comics": true,
					},
					Version: "v0.0.0",
				},
			},
			wantErr: false,
		},
		{
			name: "Error loading non allowed custom theme",
			args: args{
				options: RenderOptions{
					DisplayName:      "Test",
					InstanceStates:   instanceStates,
					Theme:            "dc-comics",
					SessionDuration:  10 * time.Minute,
					RefreshFrequency: 5 * time.Second,
					CustomThemes: fstest.MapFS{
						"marvel.html":    {Data: []byte("thor")},
						"dc-comics.html": {Data: []byte("batman")},
					},
					AllowedCustomThemes: map[string]bool{
						"marvel": true,
					},
					Version: "v0.0.0",
				},
			},
			wantErr: true,
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

func TestRenderContent(t *testing.T) {
	type args struct {
		options RenderOptions
	}
	tests := []struct {
		name        string
		args        args
		wantContent string
	}{
		{
			name: "refresh frequency is 10 seconds",
			args: args{
				options: RenderOptions{
					DisplayName:      "Test",
					InstanceStates:   instanceStates,
					Theme:            "ghost",
					SessionDuration:  10 * time.Minute,
					RefreshFrequency: 10 * time.Second,
					CustomThemes:     nil,
					Version:          "v0.0.0",
				},
			},
			wantContent: "<meta http-equiv=\"refresh\" content=\"10\" />",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writer := &bytes.Buffer{}
			if err := Render(tt.args.options, writer); err != nil {
				t.Errorf("Render() error = %v", err)
				return
			}

			content, err := io.ReadAll(writer)

			if err != nil {
				t.Errorf("ReadAll() error = %v", err)
				return
			}

			assert.Contains(t, string(content), tt.wantContent)
		})
	}
}
