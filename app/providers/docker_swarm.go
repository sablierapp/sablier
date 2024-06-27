package providers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/acouvreur/sablier/app/instance"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	log "github.com/sirupsen/logrus"
)

type DockerSwarmProvider struct {
	Client          client.APIClient
	desiredReplicas int
}

func NewDockerSwarmProvider() (*DockerSwarmProvider, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("cannot create docker client: %v", err)
	}

	serverVersion, err := cli.ServerVersion(context.Background())
	if err != nil {
		return nil, fmt.Errorf("cannot connect to docker host: %v", err)
	}

	log.Trace(fmt.Sprintf("connection established with docker %s (API %s)", serverVersion.Version, serverVersion.APIVersion))

	return &DockerSwarmProvider{
		Client:          cli,
		desiredReplicas: 1,
	}, nil

}

func (provider *DockerSwarmProvider) Start(ctx context.Context, name string) (instance.State, error) {
	return provider.scale(ctx, name, uint64(provider.desiredReplicas))
}

func (provider *DockerSwarmProvider) Stop(ctx context.Context, name string) (instance.State, error) {
	return provider.scale(ctx, name, 0)
}

func (provider *DockerSwarmProvider) scale(ctx context.Context, name string, replicas uint64) (instance.State, error) {
	service, err := provider.getServiceByName(name, ctx)

	if err != nil {
		return instance.ErrorInstanceState(name, err, provider.desiredReplicas)
	}

	foundName := provider.getInstanceName(name, *service)

	if service.Spec.Mode.Replicated == nil {
		return instance.UnrecoverableInstanceState(foundName, "swarm service is not in \"replicated\" mode", provider.desiredReplicas)
	}

	service.Spec.Mode.Replicated.Replicas = &replicas

	response, err := provider.Client.ServiceUpdate(ctx, service.ID, service.Meta.Version, service.Spec, types.ServiceUpdateOptions{})

	if err != nil {
		return instance.ErrorInstanceState(foundName, err, provider.desiredReplicas)
	}

	if len(response.Warnings) > 0 {
		return instance.UnrecoverableInstanceState(foundName, strings.Join(response.Warnings, ", "), provider.desiredReplicas)
	}

	return instance.NotReadyInstanceState(foundName, 0, provider.desiredReplicas)
}

func (provider *DockerSwarmProvider) GetGroups(ctx context.Context) (map[string][]string, error) {
	filters := filters.NewArgs()
	filters.Add("label", fmt.Sprintf("%s=true", enableLabel))

	services, err := provider.Client.ServiceList(ctx, types.ServiceListOptions{
		Filters: filters,
	})

	if err != nil {
		return nil, err
	}

	groups := make(map[string][]string)
	for _, service := range services {
		groupName := service.Spec.Labels[groupLabel]
		if len(groupName) == 0 {
			groupName = defaultGroupValue
		}

		group := groups[groupName]
		group = append(group, service.Spec.Name)
		groups[groupName] = group
	}

	return groups, nil
}

func (provider *DockerSwarmProvider) GetState(ctx context.Context, name string) (instance.State, error) {

	service, err := provider.getServiceByName(name, ctx)
	if err != nil {
		return instance.ErrorInstanceState(name, err, provider.desiredReplicas)
	}

	foundName := provider.getInstanceName(name, *service)

	if service.Spec.Mode.Replicated == nil {
		return instance.UnrecoverableInstanceState(foundName, "swarm service is not in \"replicated\" mode", provider.desiredReplicas)
	}

	if service.ServiceStatus.DesiredTasks != service.ServiceStatus.RunningTasks || service.ServiceStatus.DesiredTasks == 0 {
		return instance.NotReadyInstanceState(foundName, 0, provider.desiredReplicas)
	}

	return instance.ReadyInstanceState(foundName, provider.desiredReplicas)
}

func (provider *DockerSwarmProvider) getServiceByName(name string, ctx context.Context) (*swarm.Service, error) {
	opts := types.ServiceListOptions{
		Filters: filters.NewArgs(),
		Status:  true,
	}
	opts.Filters.Add("name", name)

	services, err := provider.Client.ServiceList(ctx, opts)

	if err != nil {
		return nil, err
	}

	if len(services) == 0 {
		return nil, fmt.Errorf(fmt.Sprintf("service with name %s was not found", name))
	}

	for _, service := range services {
		// Exact match
		if service.Spec.Name == name {
			return &service, nil
		}
	}

	return nil, fmt.Errorf(fmt.Sprintf("service %s was not found because it did not match exactly or on suffix", name))
}

func (provider *DockerSwarmProvider) getInstanceName(name string, service swarm.Service) string {
	if name == service.Spec.Name {
		return name
	}

	return fmt.Sprintf("%s (%s)", name, service.Spec.Name)
}

func (provider *DockerSwarmProvider) NotifyInstanceStopped(ctx context.Context, instance chan<- string) {
	msgs, errs := provider.Client.Events(ctx, types.EventsOptions{
		Filters: filters.NewArgs(
			filters.Arg("scope", "swarm"),
			filters.Arg("type", "service"),
		),
	})

	go func() {
		for {
			select {
			case msg, ok := <-msgs:
				if !ok {
					log.Error("provider event stream is closed")
					return
				}
				if msg.Actor.Attributes["replicas.new"] == "0" {
					instance <- msg.Actor.Attributes["name"]
				} else if msg.Action == "remove" {
					instance <- msg.Actor.Attributes["name"]
				}
			case err, ok := <-errs:
				if !ok {
					log.Error("provider event stream is closed", err)
					return
				}
				if errors.Is(err, io.EOF) {
					log.Debug("provider event stream closed")
					return
				}
				log.Error("provider event stream error", err)
			case <-ctx.Done():
				return
			}
		}
	}()
}
