package config

import (
	"fmt"
	"testing"

	"gotest.tools/v3/assert"
)

func TestProvider_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		provider Provider
		wantErr  error
	}{
		{
			name: "valid docker provider with stop strategy",
			provider: Provider{
				Name: "docker",
				Docker: Docker{
					Strategy: "stop",
				},
			},
			wantErr: nil,
		},
		{
			name: "valid docker provider with pause strategy",
			provider: Provider{
				Name: "docker",
				Docker: Docker{
					Strategy: "pause",
				},
			},
			wantErr: nil,
		},
		{
			name: "invalid docker strategy",
			provider: Provider{
				Name: "docker",
				Docker: Docker{
					Strategy: "invalid",
				},
			},
			wantErr: fmt.Errorf("unrecognized docker strategy invalid. strategies available: [stop pause]"),
		},
		{
			name: "valid kubernetes provider",
			provider: Provider{
				Name: "kubernetes",
			},
			wantErr: nil,
		},
		{
			name: "valid swarm provider",
			provider: Provider{
				Name: "swarm",
			},
			wantErr: nil,
		},
		{
			name: "valid docker_swarm provider",
			provider: Provider{
				Name: "docker_swarm",
			},
			wantErr: nil,
		},
		{
			name: "valid podman provider",
			provider: Provider{
				Name: "podman",
			},
			wantErr: nil,
		},
		{
			name: "invalid provider name",
			provider: Provider{
				Name: "invalid",
			},
			wantErr: fmt.Errorf("unrecognized provider invalid. providers available: [docker docker_swarm swarm kubernetes podman]"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.provider.IsValid()
			if tt.wantErr != nil {
				assert.Error(t, err, tt.wantErr.Error())
			} else {
				assert.NilError(t, err)
			}
		})
	}
}

func TestDocker_IsValid(t *testing.T) {
	tests := []struct {
		name    string
		docker  Docker
		wantErr error
	}{
		{
			name: "valid stop strategy",
			docker: Docker{
				Strategy: "stop",
			},
			wantErr: nil,
		},
		{
			name: "valid pause strategy",
			docker: Docker{
				Strategy: "pause",
			},
			wantErr: nil,
		},
		{
			name: "invalid strategy",
			docker: Docker{
				Strategy: "restart",
			},
			wantErr: fmt.Errorf("unrecognized docker strategy restart. strategies available: [stop pause]"),
		},
		{
			name: "empty strategy",
			docker: Docker{
				Strategy: "",
			},
			wantErr: fmt.Errorf("unrecognized docker strategy . strategies available: [stop pause]"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.docker.IsValid()
			if tt.wantErr != nil {
				assert.Error(t, err, tt.wantErr.Error())
			} else {
				assert.NilError(t, err)
			}
		})
	}
}
