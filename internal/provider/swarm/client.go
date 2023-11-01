package swarm

import (
	"context"
	"fmt"

	"github.com/acouvreur/sablier/internal/provider"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
)

type ClientWrapper interface {
	ServiceList(ctx context.Context, options types.ServiceListOptions) ([]swarm.Service, error)
	ServiceInspectWithRaw(ctx context.Context, serviceID string, options types.ServiceInspectOptions) (swarm.Service, []byte, error)
	ServiceUpdate(ctx context.Context, serviceID string, version swarm.Version, service swarm.ServiceSpec, options types.ServiceUpdateOptions) (types.ServiceUpdateResponse, error)
	Events(ctx context.Context, options types.EventsOptions) (<-chan events.Message, <-chan error)
}

type Client struct {
	Client ClientWrapper
}

func NewSwarmClient() (*Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	// cluster, err := cli.SwarmInspect(context.TODO())
	// if err != nil {
	// 	return nil, err
	// }

	// TODO: Check that we are running from a manager node

	return &Client{
		Client: cli,
	}, nil
}

func (client *Client) Start(ctx context.Context, name string, opts provider.StartOptions) error {
	service, _, err := client.Client.ServiceInspectWithRaw(ctx, name, types.ServiceInspectOptions{})
	if err != nil {
		return err
	}

	if service.Spec.Mode.Replicated == nil {
		return fmt.Errorf("service \"%s\" is not in \"replicated\" mode", service.Spec.Name)
	}

	spec := service.Spec
	spec.Mode.Replicated.Replicas = replicas(opts.DesiredReplicas)

	_, err = client.Client.ServiceUpdate(ctx, service.ID, service.Meta.Version, spec, types.ServiceUpdateOptions{})
	if err != nil {
		return err
	}

	return nil
}

func (client *Client) Stop(ctx context.Context, name string) error {
	service, _, err := client.Client.ServiceInspectWithRaw(ctx, name, types.ServiceInspectOptions{})
	if err != nil {
		return err
	}

	if service.Spec.Mode.Replicated == nil {
		return fmt.Errorf("service \"%s\" is not in \"replicated\" mode", service.Spec.Name)
	}

	spec := service.Spec
	spec.Mode.Replicated.Replicas = replicas(0)

	_, err = client.Client.ServiceUpdate(ctx, service.ID, service.Meta.Version, spec, types.ServiceUpdateOptions{})
	if err != nil {
		return err
	}

	return nil
}

func (client *Client) Status(ctx context.Context, name string) (bool, error) {
	service, _, err := client.Client.ServiceInspectWithRaw(ctx, name, types.ServiceInspectOptions{})
	if err != nil {
		return false, err
	}

	if service.Spec.Mode.Replicated == nil {
		return false, fmt.Errorf("service \"%s\" is not in \"replicated\" mode", service.Spec.Name)
	}

	ready := service.UpdateStatus.State == swarm.UpdateStateCompleted

	return ready, nil
}

func replicas(replicas uint32) *uint64 {
	var value uint64 = uint64(replicas)
	return &value
}

func (client *Client) Discover(ctx context.Context, opts provider.DiscoveryOptions) ([]provider.Discovered, error) {
	filters := filters.NewArgs()
	filters.Add("label", fmt.Sprintf("%s=true", opts.EnableLabel))

	services, err := client.Client.ServiceList(ctx, types.ServiceListOptions{
		Filters: filters,
	})
	if err != nil {
		return nil, err
	}

	discovered := make([]provider.Discovered, len(services))
	for index, service := range services {
		discovered[index] = toDiscovered(service, opts)
	}

	return discovered, nil
}

func toDiscovered(service swarm.Service, opts provider.DiscoveryOptions) provider.Discovered {
	name := service.Spec.Name
	var group string

	// The service defined a label with its named group
	if foundGroup, ok := service.Spec.Labels[opts.GroupLabel]; ok {
		group = foundGroup
	} else if opts.DefaultGroupStrategy == provider.DefaultGroupStrategyUseInstanceName {
		// The service did not define a label and uses the instance name as group
		group = name
	} else if opts.DefaultGroupStrategy == provider.DefaultGroupStrategyUseValue {
		// The service did not define a label and uses the "default" group
		group = provider.DefaultGroupValue
	}

	return provider.Discovered{
		Name:  name,
		Group: group,
	}
}
