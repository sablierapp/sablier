package scaler

import (
	"context"
	"fmt"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	log "github.com/sirupsen/logrus"
)

type DockerSwarmScaler struct {
	Client client.ServiceAPIClient
}

func NewDockerSwarmScaler() *DockerSwarmScaler {
	return &DockerSwarmScaler{}
}

func (scaler *DockerSwarmScaler) ScaleUp(name string) error {
	ctx := context.Background()
	service, err := scaler.GetServiceByName(name, ctx)

	if err != nil {
		log.Error(err.Error())
		return err
	}

	if *service.Spec.Mode.Replicated.Replicas == onereplicas {
		log.Infof("%s already scaled up to %d", name, onereplicas)
		return nil
	}
	log.Infof("scaling up %s to %d", name, onereplicas)

	service.Spec.Mode.Replicated = &swarm.ReplicatedService{
		Replicas: &onereplicas,
	}
	response, err := scaler.Client.ServiceUpdate(ctx, service.ID, service.Meta.Version, service.Spec, types.ServiceUpdateOptions{})

	if err != nil {
		log.Error(err.Error())
		return err
	}

	if len(response.Warnings) > 0 {
		log.Warnf("received scaling up service %s: %v", name, response.Warnings)
	}

	return nil
}

func (scaler *DockerSwarmScaler) ScaleDown(name string) error {
	ctx := context.Background()
	service, err := scaler.GetServiceByName(name, ctx)

	if err != nil {
		log.Error(err.Error())
		return err
	}

	replicas := uint64(0)

	if *service.Spec.Mode.Replicated.Replicas == replicas {
		log.Infof("%s already scaled down to %d", name, replicas)
		return nil
	}
	log.Infof("scaling down %s to %d", name, replicas)

	service.Spec.Mode.Replicated = &swarm.ReplicatedService{
		Replicas: &replicas,
	}
	response, err := scaler.Client.ServiceUpdate(ctx, service.ID, service.Meta.Version, service.Spec, types.ServiceUpdateOptions{})

	if err != nil {
		log.Error(err.Error())
		return err
	}

	if len(response.Warnings) > 0 {
		log.Warnf("received scaling up service %s: %v", name, response.Warnings)
	}

	return nil
}

func (scaler *DockerSwarmScaler) IsUp(name string) bool {
	ctx := context.Background()
	service, err := scaler.GetServiceByName(name, ctx)

	if err != nil {
		log.Error(err.Error())
		return false
	}

	return scaler.isServiceRunningFor(service, 5*time.Second)
}

func (scaler *DockerSwarmScaler) isServiceRunningFor(service *swarm.Service, duration time.Duration) bool {

	if service.ServiceStatus.DesiredTasks == 0 {
		return false
	}

	if service.ServiceStatus.DesiredTasks != service.ServiceStatus.RunningTasks {
		return false
	}

	opts := types.TaskListOptions{
		Filters: filters.NewArgs(),
	}
	opts.Filters.Add("desired-state", "running")
	opts.Filters.Add("service", service.Spec.Name)

	ctx := context.Background()
	tasks, err := scaler.Client.TaskList(ctx, opts)
	if err != nil {
		log.Error(err.Error())
		return false
	}

	if len(tasks) == 0 {
		log.Error("No task found with filter desired-state=running and service=", service.Spec.Name)
		return false
	}

	return true
}

func (scaler *DockerSwarmScaler) GetServiceByName(name string, ctx context.Context) (*swarm.Service, error) {
	opts := types.ServiceListOptions{
		Filters: filters.NewArgs(),
		Status:  true,
	}
	opts.Filters.Add("name", name)

	services, err := scaler.Client.ServiceList(ctx, opts)

	if err != nil {
		return nil, err
	}

	if len(services) == 0 {
		return nil, fmt.Errorf(fmt.Sprintf("service with name %s was not found", name))
	}

	if len(services) > 1 {
		return nil, fmt.Errorf("multiple services (%d) with name %s were found: %v", len(services), name, services)
	}

	return &services[0], nil
}
