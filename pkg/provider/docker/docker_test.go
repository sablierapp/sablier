package docker

import (
	"context"
	"github.com/docker/docker/api/types/container"
	"github.com/google/go-cmp/cmp"
	"github.com/neilotoole/slogt"
	"gotest.tools/v3/assert"
	"reflect"
	"testing"
	"time"

	"github.com/sablierapp/sablier/app/instance"
)

// 			Cmd:    []string{"/mimic", "-running", "-running-after", opts.RunningAfter.String(), "-healthy=false"},

// &container.HealthConfig{
//			Test:          []string{"CMD", "/mimic", "healthcheck"},
//			Interval:      100 * time.Millisecond,
//			Timeout:       time.Second,
//			StartPeriod:   opts.RunningAfter,
//			StartInterval: time.Second,
//			Retries:       50,
//		}

func TestDockerClassicProvider_GetState(t *testing.T) {
	ctx := context.Background()
	type args struct {
		do func(dind *dindContainer) (string, error)
	}
	tests := []struct {
		name    string
		args    args
		want    instance.State
		wantErr error
	}{
		{
			name: "created container",
			args: args{
				do: func(dind *dindContainer) (string, error) {
					resp, err := dind.CreateMimic(ctx, MimicOptions{
						Cmd:         []string{"/mimic"},
						Healthcheck: nil,
					})
					return resp.ID, err
				},
			},
			want: instance.State{
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          instance.NotReady,
			},
			wantErr: nil,
		},
		{
			name: "running container without healthcheck",
			args: args{
				do: func(dind *dindContainer) (string, error) {
					c, err := dind.CreateMimic(ctx, MimicOptions{
						Cmd:         []string{"/mimic"},
						Healthcheck: nil,
					})
					if err != nil {
						return "", err
					}

					return c.ID, dind.client.ContainerStart(ctx, c.ID, container.StartOptions{})
				},
			},
			want: instance.State{
				CurrentReplicas: 1,
				DesiredReplicas: 1,
				Status:          instance.Ready,
			},
			wantErr: nil,
		},
		{
			name: "running container with \"starting\" health",
			args: args{
				do: func(dind *dindContainer) (string, error) {
					c, err := dind.CreateMimic(ctx, MimicOptions{
						Cmd: []string{"/mimic", "-running", "-running-after", "1s", "-healthy", "true"},
						// Keep long interval so that the container is still in starting state
						Healthcheck: &container.HealthConfig{
							Test:          []string{"CMD", "/mimic", "healthcheck"},
							Interval:      time.Second,
							Timeout:       10 * time.Second,
							StartPeriod:   10 * time.Second,
							StartInterval: 10 * time.Second,
							Retries:       10,
						},
					})
					if err != nil {
						return "", err
					}

					return c.ID, dind.client.ContainerStart(ctx, c.ID, container.StartOptions{})
				},
			},
			want: instance.State{
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          instance.NotReady,
			},
			wantErr: nil,
		},
		{
			name: "running container with \"unhealthy\" health",
			args: args{
				do: func(dind *dindContainer) (string, error) {
					c, err := dind.CreateMimic(ctx, MimicOptions{
						Cmd: []string{"/mimic", "-running", "-running-after=1ms", "-healthy=false", "-healthy-after=1ms"},
						Healthcheck: &container.HealthConfig{
							Test:          []string{"CMD", "/mimic", "healthcheck"},
							Timeout:       time.Second,
							Interval:      time.Millisecond,
							StartInterval: time.Millisecond,
							StartPeriod:   time.Millisecond,
							Retries:       1,
						},
					})
					if err != nil {
						return "", err
					}

					err = dind.client.ContainerStart(ctx, c.ID, container.StartOptions{})
					if err != nil {
						return "", err
					}

					<-time.After(2 * time.Second)

					return c.ID, nil
				},
			},
			want: instance.State{
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          instance.Unrecoverable,
				Message:         "container is unhealthy",
			},
			wantErr: nil,
		},
		{
			name: "running container with \"healthy\" health",
			args: args{
				do: func(dind *dindContainer) (string, error) {
					c, err := dind.CreateMimic(ctx, MimicOptions{
						Cmd: []string{"/mimic", "-running", "-running-after=1ms", "-healthy", "-healthy-after=1ms"},
						Healthcheck: &container.HealthConfig{
							Test:          []string{"CMD", "/mimic", "healthcheck"},
							Interval:      100 * time.Millisecond,
							Timeout:       time.Second,
							StartPeriod:   time.Second,
							StartInterval: 100 * time.Millisecond,
							Retries:       10,
						},
					})
					if err != nil {
						return "", err
					}

					err = dind.client.ContainerStart(ctx, c.ID, container.StartOptions{})
					if err != nil {
						return "", err
					}

					<-time.After(2 * time.Second)

					return c.ID, nil
				},
			},
			want: instance.State{
				CurrentReplicas: 1,
				DesiredReplicas: 1,
				Status:          instance.Ready,
			},
			wantErr: nil,
		},
		{
			name: "paused container",
			args: args{
				do: func(dind *dindContainer) (string, error) {
					c, err := dind.CreateMimic(ctx, MimicOptions{})
					if err != nil {
						return "", err
					}

					err = dind.client.ContainerStart(ctx, c.ID, container.StartOptions{})
					if err != nil {
						return "", err
					}

					err = dind.client.ContainerPause(ctx, c.ID)
					if err != nil {
						return "", err
					}

					return c.ID, nil
				},
			},
			want: instance.State{
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          instance.NotReady,
			},
			wantErr: nil,
		},
		{
			name: "restarting container",
			args: args{
				do: func(dind *dindContainer) (string, error) {
					c, err := dind.CreateMimic(ctx, MimicOptions{
						Cmd: []string{"/mimic", "-running=false", "-exit-code=1"},
						RestartPolicy: container.RestartPolicy{
							Name: container.RestartPolicyAlways,
						},
					})
					if err != nil {
						return "", err
					}

					err = dind.client.ContainerStart(ctx, c.ID, container.StartOptions{})
					if err != nil {
						return "", err
					}

					<-time.After(2 * time.Second)

					return c.ID, nil
				},
			},
			want: instance.State{
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          instance.NotReady,
			},
			wantErr: nil,
		},
		{
			name: "exited container with status code 0",
			args: args{
				do: func(dind *dindContainer) (string, error) {
					c, err := dind.CreateMimic(ctx, MimicOptions{
						Cmd: []string{"/mimic", "-running=false", "-exit-code=0"},
					})
					if err != nil {
						return "", err
					}

					err = dind.client.ContainerStart(ctx, c.ID, container.StartOptions{})
					if err != nil {
						return "", err
					}

					return c.ID, nil
				},
			},
			want: instance.State{
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          instance.NotReady,
			},
			wantErr: nil,
		},
		{
			name: "nginx exited container state with status code 137",
			args: args{
				do: func(dind *dindContainer) (string, error) {
					c, err := dind.CreateMimic(ctx, MimicOptions{
						Cmd: []string{"/mimic", "-running=false", "-exit-code=137"},
					})
					if err != nil {
						return "", err
					}

					err = dind.client.ContainerStart(ctx, c.ID, container.StartOptions{})
					if err != nil {
						return "", err
					}

					return c.ID, nil
				},
			},
			want: instance.State{
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          instance.Unrecoverable,
				Message:         "container exited with code \"137\"",
			},
			wantErr: nil,
		},
	}
	c := setupDinD(t, ctx)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p, err := NewDockerClassicProvider(ctx, c.client, slogt.New(t))

			name, err := tt.args.do(c)
			assert.NilError(t, err)

			tt.want.Name = name
			got, err := p.GetState(ctx, name)
			if !cmp.Equal(err, tt.wantErr) {
				t.Errorf("DockerClassicProvider.GetState() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DockerClassicProvider.GetState() = %v, want %v", got, tt.want)
			}
		})
	}
}

/*
func TestDockerClassicProvider_Stop(t *testing.T) {
	type fields struct {
		Client *mocks.DockerAPIClientMock
	}
	type args struct {
		name string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
		err     error
	}{
		{
			name: "container stop has an error",
			fields: fields{
				Client: mocks.NewDockerAPIClientMock(),
			},
			args: args{
				name: "nginx",
			},
			wantErr: true,
			err:     fmt.Errorf("container with name \"nginx\" was not found"),
		},
		{
			name: "container stop as expected",
			fields: fields{
				Client: mocks.NewDockerAPIClientMock(),
			},
			args: args{
				name: "nginx",
			},
			wantErr: false,
			err:     nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := setupProvider(t, tt.fields.Client)

			tt.fields.Client.On("ContainerStop", mock.Anything, mock.Anything, mock.Anything).Return(tt.err)

			err := provider.Stop(context.Background(), tt.args.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("DockerClassicProvider.Stop() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestDockerClassicProvider_Start(t *testing.T) {
	type fields struct {
		Client *mocks.DockerAPIClientMock
	}
	type args struct {
		name string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
		err     error
	}{
		{
			name: "container start has an error",
			fields: fields{
				Client: mocks.NewDockerAPIClientMock(),
			},
			args: args{
				name: "nginx",
			},
			wantErr: true,
			err:     fmt.Errorf("container with name \"nginx\" was not found"),
		},
		{
			name: "container start as expected",
			fields: fields{
				Client: mocks.NewDockerAPIClientMock(),
			},
			args: args{
				name: "nginx",
			},
			wantErr: false,
			err:     nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := setupProvider(t, tt.fields.Client)

			tt.fields.Client.On("ContainerStart", mock.Anything, mock.Anything, mock.Anything).Return(tt.err)

			err := provider.Start(context.Background(), tt.args.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("DockerClassicProvider.Start() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestDockerClassicProvider_NotifyInstanceStopped(t *testing.T) {
	tests := []struct {
		name   string
		want   []string
		events []events.Message
		errors []error
	}{
		{
			name: "container nginx is stopped",
			want: []string{"nginx"},
			events: []events.Message{
				mocks.ContainerStoppedEvent("nginx"),
			},
			errors: []error{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := setupProvider(t, mocks.NewDockerAPIClientMockWithEvents(tt.events, tt.errors))

			instanceC := make(chan string, 1)

			ctx, cancel := context.WithCancel(context.Background())
			provider.NotifyInstanceStopped(ctx, instanceC)

			var got []string

			got = append(got, <-instanceC)
			cancel()
			close(instanceC)

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NotifyInstanceStopped() = %v, want %v", got, tt.want)
			}
		})
	}
}
*/
