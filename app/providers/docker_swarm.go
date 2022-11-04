package providers

import (
	"context"
	"fmt"
	"strings"

	"github.com/acouvreur/sablier/app/instance"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	log "github.com/sirupsen/logrus"
)

type DockerSwarmProvider struct {
	Client          client.ServiceAPIClient
	desiredReplicas int
}

func NewDockerSwarmProvider() (*DockerSwarmProvider, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &DockerSwarmProvider{
		Client:          cli,
		desiredReplicas: 1,
	}, nil
}

func (provider *DockerSwarmProvider) Start(name string) (instance.State, error) {
	return provider.scale(name, uint64(provider.desiredReplicas))
}

func (provider *DockerSwarmProvider) Stop(name string) (instance.State, error) {
	return provider.scale(name, 0)
}

func (provider *DockerSwarmProvider) scale(name string, replicas uint64) (instance.State, error) {
	ctx := context.Background()
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

func (provider *DockerSwarmProvider) GetState(name string) (instance.State, error) {
	ctx := context.Background()

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

	suffixMatches := make([]swarm.Service, 0)
	suffixMatchNames := make([]string, 0)

	for _, service := range services {
		// Exact match
		if service.Spec.Name == name {
			return &service, nil
		} else if strings.HasSuffix(service.Spec.Name, name) {
			suffixMatches = append(suffixMatches, service)
			suffixMatchNames = append(suffixMatchNames, service.Spec.Name)
		} else {
			log.Warnf("service %s was ignored because it did not match %s exactly or on suffix", service.Spec.Name, name)
		}
	}

	if len(suffixMatches) > 1 {
		return nil, fmt.Errorf("ambiguous service names found for \"%s\" (%s)", name, strings.Join(suffixMatchNames, ", "))
	}

	if len(suffixMatches) == 1 {
		return &suffixMatches[0], nil
	}

	return nil, fmt.Errorf(fmt.Sprintf("service %s was not found because it did not match exactly or on suffix", name))
}

func (provider *DockerSwarmProvider) getInstanceName(name string, service swarm.Service) string {
	if name == service.Spec.Name {
		return name
	}

	return fmt.Sprintf("%s (%s)", name, service.Spec.Name)
}

func (provider *DockerSwarmProvider) NotifyInsanceStopped(ctx context.Context, instance chan string) {
}
