package providers

import (
	"context"
	"fmt"
	"reflect"
	"testing"
)

func TestNomadProvider_Start(t *testing.T) {
	t.Skip() // TODO: mock these
	provider, _ := NewNomadProvider()
	_, err := provider.Start(context.Background(), "whoami@default/webui")
	if err != nil {
		t.Errorf("NomadProvider.Start() error = %v", err)
	}
}

func TestNomadProvider_GetState(t *testing.T) {
	t.Skip() // TODO: mock these
	provider, _ := NewNomadProvider()
	s, err := provider.GetState(context.Background(), "whoami@default/webui")
	if err != nil {
		t.Errorf("NomadProvider.GetState() error = %v", err)
	}

	fmt.Println(s)
}

func TestNomadProvider_Stop(t *testing.T) {
	t.Skip() // TODO: mock these
	provider, _ := NewNomadProvider()
	s, err := provider.Stop(context.Background(), "whoami@default/webui")
	if err != nil {
		t.Errorf("NomadProvider.Start() error = %v", err)
	}
	fmt.Println(s)
}

func TestNomadProvider_convertName(t *testing.T) {
	type args struct {
		name string
	}
	tests := []struct {
		name    string
		args    args
		want    *NomadConfig
		wantErr bool
	}{
		{
			name: "default",
			args: args{name: "job@namespace/taskgroup"},
			want: &NomadConfig{
				OriginalName: "job@namespace/taskgroup",
				Job:          "job",
				Namespace:    "namespace",
				Group:        "taskgroup",
				Replicas:     1,
			},
			wantErr: false,
		},
		{
			name: "custom replicas",
			args: args{name: "job@namespace/taskgroup/4"},
			want: &NomadConfig{
				OriginalName: "job@namespace/taskgroup/4",
				Job:          "job",
				Namespace:    "namespace",
				Group:        "taskgroup",
				Replicas:     4,
			},
			wantErr: false,
		},
		{
			name: "group has dashes",
			args: args{name: "test@default/task-group/1"},
			want: &NomadConfig{
				OriginalName: "test@default/task-group/1",
				Job:          "test",
				Namespace:    "default",
				Group:        "task-group",
				Replicas:     1,
			},
			wantErr: false,
		},
		{
			name: "has lots of dashes",
			args: args{name: "hello-world@default/hello-world/1"},
			want: &NomadConfig{
				OriginalName: "hello-world@default/hello-world/1",
				Job:          "hello-world",
				Namespace:    "default",
				Group:        "hello-world",
				Replicas:     1,
			},
			wantErr: false,
		},
		{
			name: "invalid group",
			args: args{name: "job@namespace_without_group"},
			want: &NomadConfig{
				OriginalName: "job@namespace_without_group",
				Job:          "",
				Namespace:    "",
				Group:        "",
				Replicas:     1,
			},
			wantErr: true,
		},
		{
			name: "invalid namespace",
			args: args{name: "job_without_namespace/taskgroup"},
			want: &NomadConfig{
				OriginalName: "job_without_namespace/taskgroup",
				Job:          "",
				Namespace:    "",
				Group:        "taskgroup",
				Replicas:     1,
			},
			wantErr: true,
		},
		{
			name: "invalid replicas",
			args: args{name: "job@with/invalid_replicas/one"},
			want: &NomadConfig{
				OriginalName: "job@with/invalid_replicas/one",
				Job:          "job",
				Namespace:    "with",
				Group:        "invalid_replicas",
				Replicas:     0,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &NomadProvider{}
			got, err := provider.convertName(tt.args.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("NomadProvider.convertName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NomadProvider.convertName() = %v, want %v", got, tt.want)
			}
		})
	}
}
