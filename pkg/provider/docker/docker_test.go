package docker

import (
	"bytes"
	"context"
	"fmt"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/sablierapp/sablier/app/instance"
)

type dindContainer struct {
	testcontainers.Container
	client *client.Client
}

// TODO: Every provider should implement tests against a testcontainer for accurate and
// up to date behaviors.
// Complete this test.
// Should I do it for sure ? Will it be flaky ? It's like testing against a mocked database, it is pointless.
// Better test against real docker socket.

type MimicOptions struct {
	Name         string
	WithHealth   bool
	HealthyAfter time.Duration
	RunningAfter time.Duration
	Registered   bool
	SablierGroup string
}

func (d *dindContainer) CreateMimic(ctx context.Context, opts MimicOptions) (container.CreateResponse, error) {
	/*i, err := d.client.ImagePull(ctx, "docker.io/sablierapp/mimic:v0.3.1", image.PullOptions{})
	if err != nil {
		return container.CreateResponse{}, err
	}
	_, err = d.client.ImageLoad(ctx, i, false)
	if err != nil {
		return container.CreateResponse{}, err
	}*/

	labels := make(map[string]string)
	if opts.Registered == true {
		labels["sablier.enable"] = "true"
		if opts.SablierGroup != "" {
			labels["sablier.group"] = opts.SablierGroup
		}
	}

	if opts.WithHealth == false {
		return d.client.ContainerCreate(ctx, &container.Config{
			Cmd:    []string{"/mimic", "-running", "-running-after", opts.RunningAfter.String(), "-healthy=false"},
			Image:  "sablierapp/mimic:v0.3.1",
			Labels: labels,
		}, nil, nil, nil, opts.Name)
	}
	return d.client.ContainerCreate(ctx, &container.Config{
		Cmd: []string{"/mimic", "-running", "-running-after", opts.RunningAfter.String(), "-healthy", "--healthy-after", opts.HealthyAfter.String()},
		Healthcheck: &container.HealthConfig{
			Test:          []string{"CMD", "/mimic", "healthcheck"},
			Interval:      100 * time.Millisecond,
			Timeout:       time.Second,
			StartPeriod:   opts.RunningAfter,
			StartInterval: time.Second,
			Retries:       50,
		},
		Image:  "sablierapp/mimic:v0.3.1",
		Labels: labels,
	}, nil, nil, nil, opts.Name)
}

func setupDinD(t *testing.T, ctx context.Context) (*dindContainer, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	req := testcontainers.ContainerRequest{
		Image:        "docker:dind",
		ExposedPorts: []string{"2375/tcp"},
		WaitingFor:   wait.ForLog("API listen on [::]:2375"),
		Cmd: []string{
			"dockerd", "-H", "tcp://0.0.0.0:2375", "--tls=false",
		},
		Privileged: true,
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
		Logger:           testcontainers.TestLogger(t),
	})
	if err != nil {
		return nil, err
	}
	t.Cleanup(func() {
		testcontainers.CleanupContainer(t, c)
	})

	ip, err := c.Host(ctx)
	if err != nil {
		return nil, err
	}

	mappedPort, err := c.MappedPort(ctx, "2375")
	if err != nil {
		return nil, err
	}

	host := fmt.Sprintf("http://%s:%s", ip, mappedPort.Port())
	dindCli, err := client.NewClientWithOpts(client.WithHost(host), client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker in docker client: %w", err)
	}

	err = addMimicToDind(ctx, cli, dindCli)
	if err != nil {
		return nil, fmt.Errorf("failed to add mimic to dind: %w", err)
	}

	return &dindContainer{
		Container: c,
		client:    dindCli,
	}, nil
}

func searchMimicImage(ctx context.Context, cli *client.Client) (string, error) {
	images, err := cli.ImageList(ctx, image.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to list images: %w", err)
	}

	for _, summary := range images {
		if slices.Contains(summary.RepoTags, "sablierapp/mimic:v0.3.1") {
			return summary.ID, nil
		}
	}

	return "", nil
}

func pullMimicImage(ctx context.Context, cli *client.Client) error {
	reader, err := cli.ImagePull(ctx, "sablierapp/mimic:v0.3.1", image.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}
	defer reader.Close()
	resp, err := cli.ImageLoad(ctx, reader, true)
	if err != nil {
		return fmt.Errorf("failed to load image: %w", err)
	}
	defer resp.Body.Close()
	return nil
}

func addMimicToDind(ctx context.Context, cli *client.Client, dindCli *client.Client) error {
	ID, err := searchMimicImage(ctx, cli)
	if err != nil {
		return fmt.Errorf("failed to search for mimic image: %w", err)
	}

	if ID == "" {
		err = pullMimicImage(ctx, cli)
		if err != nil {
			return err
		}

		ID, err = searchMimicImage(ctx, cli)
		if err != nil {
			return fmt.Errorf("failed to search for mimic image even though it's just been pulled without errors: %w", err)
		}
	}

	reader, err := cli.ImageSave(ctx, []string{ID})
	if err != nil {
		return fmt.Errorf("failed to save image: %w", err)
	}
	defer reader.Close()

	resp, err := dindCli.ImageLoad(ctx, reader, true)
	if err != nil {
		return fmt.Errorf("failed to load image in docker in docker container: %w", err)
	}
	defer resp.Body.Close()
	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read from response body: %w", err)
	}

	list, err := dindCli.ImageList(ctx, image.ListOptions{})
	if err != nil {
		return err
	}

	err = dindCli.ImageTag(ctx, list[0].ID, "sablierapp/mimic:v0.3.1")
	if err != nil {
		return err
	}

	return nil
}

func TestDockerClassicProvider_GetState(t *testing.T) {
	ctx := context.Background()
	type args struct {
		do   func(dind *dindContainer) error
		name string
	}
	tests := []struct {
		name    string
		args    args
		want    instance.State
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "nginx created container state",
			args: args{
				do: func(dind *dindContainer) error {
					_, err := dind.CreateMimic(ctx, MimicOptions{
						Name: "test-info-created",
					})
					return err
				},
				name: "nginx",
			},
			want: instance.State{
				Name:            "nginx",
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          instance.NotReady,
			},
			wantErr: assert.NoError,
		},
		{
			name: "nginx running container state without healthcheck",
			args: args{
				name: "nginx",
			},
			want: instance.State{
				Name:            "nginx",
				CurrentReplicas: 1,
				DesiredReplicas: 1,
				Status:          instance.Ready,
			},
			wantErr:       false,
			containerSpec: mocks.RunningWithoutHealthcheckContainerSpec("nginx"),
		},
		{
			name: "nginx running container state with \"starting\" health",
			args: args{
				name: "nginx",
			},
			want: instance.State{
				Name:            "nginx",
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          instance.NotReady,
			},
			wantErr:       false,
			containerSpec: mocks.RunningWithHealthcheckContainerSpec("nginx", "starting"),
		},
		{
			name: "nginx running container state with \"unhealthy\" health",
			args: args{
				name: "nginx",
			},
			want: instance.State{
				Name:            "nginx",
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          instance.Unrecoverable,
				Message:         "container is unhealthy: curl http://localhost failed (1)",
			},
			wantErr:       false,
			containerSpec: mocks.RunningWithHealthcheckContainerSpec("nginx", "unhealthy"),
		},
		{
			name: "nginx running container state with \"healthy\" health",
			args: args{
				name: "nginx",
			},
			want: instance.State{
				Name:            "nginx",
				CurrentReplicas: 1,
				DesiredReplicas: 1,
				Status:          instance.Ready,
			},
			wantErr:       false,
			containerSpec: mocks.RunningWithHealthcheckContainerSpec("nginx", "healthy"),
		},
		{
			name: "nginx paused container state",
			args: args{
				name: "nginx",
			},
			want: instance.State{
				Name:            "nginx",
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          instance.NotReady,
			},
			wantErr:       false,
			containerSpec: mocks.PausedContainerSpec("nginx"),
		},
		{
			name: "nginx restarting container state",
			args: args{
				name: "nginx",
			},
			want: instance.State{
				Name:            "nginx",
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          instance.NotReady,
			},
			wantErr:       false,
			containerSpec: mocks.RestartingContainerSpec("nginx"),
		},
		{
			name: "nginx removing container state",
			args: args{
				name: "nginx",
			},
			want: instance.State{
				Name:            "nginx",
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          instance.NotReady,
			},
			wantErr:       false,
			containerSpec: mocks.RemovingContainerSpec("nginx"),
		},
		{
			name: "nginx exited container state with status code 0",
			args: args{
				name: "nginx",
			},
			want: instance.State{
				Name:            "nginx",
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          instance.NotReady,
			},
			wantErr:       false,
			containerSpec: mocks.ExitedContainerSpec("nginx", 0),
		},
		{
			name: "nginx exited container state with status code 137",
			args: args{
				name: "nginx",
			},
			want: instance.State{
				Name:            "nginx",
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          instance.Unrecoverable,
				Message:         "container exited with code \"137\"",
			},
			wantErr:       false,
			containerSpec: mocks.ExitedContainerSpec("nginx", 137),
		},
		{
			name: "nginx dead container state",
			args: args{
				name: "nginx",
			},
			want: instance.State{
				Name:            "nginx",
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Status:          instance.Unrecoverable,
				Message:         "container in \"dead\" state cannot be restarted",
			},
			wantErr:       false,
			containerSpec: mocks.DeadContainerSpec("nginx"),
		},
		{
			name: "container inspect has an error",
			args: args{
				name: "nginx",
			},
			want:          instance.State{},
			wantErr:       true,
			containerSpec: types.ContainerJSON{},
			err:           fmt.Errorf("container with name \"nginx\" was not found"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, provider := setupProvider(t)

			client.On("ContainerInspect", mock.Anything, mock.Anything).Return(tt.containerSpec, tt.err)

			got, err := provider.GetState(context.Background(), tt.args.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("DockerClassicProvider.GetState() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DockerClassicProvider.GetState() = %v, want %v", got, tt.want)
			}
		})
	}
}

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
