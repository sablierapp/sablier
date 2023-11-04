package docker

import (
	"context"
	"fmt"
	"strings"

	"github.com/acouvreur/sablier/internal/provider"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

type ClientWrapper interface {
	ContainerStart(ctx context.Context, container string, options types.ContainerStartOptions) error
	ContainerStop(ctx context.Context, container string, options container.StopOptions) error
	ContainerInspect(ctx context.Context, container string) (types.ContainerJSON, error)
	ContainerList(ctx context.Context, options types.ContainerListOptions) ([]types.Container, error)
	Events(ctx context.Context, options types.EventsOptions) (<-chan events.Message, <-chan error)
}

type Client struct {
	Client ClientWrapper
}

func NewDockerClient() (*Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	return &Client{
		Client: cli,
	}, nil
}

func (client *Client) Start(ctx context.Context, name string, opts provider.StartOptions) error {
	return client.Client.ContainerStart(ctx, name, types.ContainerStartOptions{})
}

func (client *Client) Stop(ctx context.Context, name string) error {
	return client.Client.ContainerStop(ctx, name, container.StopOptions{})
}

func (client *Client) Status(ctx context.Context, name string) (bool, error) {
	c, err := client.Client.ContainerInspect(ctx, name)
	if err != nil {
		return false, err
	}

	if c.State.Status != "running" {
		return false, nil
	}

	if c.State.Health != nil {
		return c.State.Health.Status == "healthy", nil
	}

	return true, nil
}

func (client *Client) Discover(ctx context.Context, opts provider.DiscoveryOptions) ([]provider.Discovered, error) {
	f := filters.NewArgs()
	f.Add("label", fmt.Sprintf("%s=true", opts.EnableLabel))

	containers, err := client.Client.ContainerList(ctx, types.ContainerListOptions{
		All:     true,
		Filters: f,
	})
	if err != nil {
		return nil, err
	}

	discovered := make([]provider.Discovered, len(containers))
	for index, c := range containers {
		discovered[index] = toDiscovered(c, opts)
	}

	return discovered, nil
}

func toDiscovered(container types.Container, opts provider.DiscoveryOptions) provider.Discovered {
	name, _ := strings.CutPrefix(container.Names[0], "/")
	var group string

	// The container defined a label with it's named group
	if foundGroup, ok := container.Labels[opts.GroupLabel]; ok {
		group = foundGroup
	} else if opts.DefaultGroupStrategy == provider.DefaultGroupStrategyUseInstanceName {
		// The container did not define a label and uses the instance name as group
		group = name
	} else if opts.DefaultGroupStrategy == provider.DefaultGroupStrategyUseValue {
		// The container did not define a label and uses the "default" group
		group = provider.DefaultGroupValue
	}

	return provider.Discovered{
		Name:  name,
		Group: group,
	}
}
