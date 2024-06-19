package providers

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/acouvreur/sablier/app/instance"
	log "github.com/sirupsen/logrus"

	nomadapi "github.com/hashicorp/nomad/api"
)

type NomadProvider struct {
	Client *nomadapi.Client
}

type NomadConfig struct {
	OriginalName string
	Namespace    string
	Job          string
	Group        string
	Replicas     int
}

func NewNomadProvider() (*NomadProvider, error) {

	config := nomadapi.DefaultConfig()

	client, err := nomadapi.NewClient(config)
	if err != nil {
		return nil, err
	}

	return &NomadProvider{
		Client: client,
	}, nil
}

func (provider *NomadProvider) Start(ctx context.Context, name string) (instance.State, error) {

	// parse config from name
	config, err := provider.convertName(name)
	if err != nil {
		return instance.ErrorInstanceState(name, err, config.Replicas)
	}

	return provider.scale(config, config.Replicas)
}

func (provider *NomadProvider) Stop(ctx context.Context, name string) (instance.State, error) {
	// parse config from name
	config, err := provider.convertName(name)
	if err != nil {
		return instance.ErrorInstanceState(name, err, config.Replicas)
	}

	return provider.scale(config, 0)
}

func (provider *NomadProvider) GetState(ctx context.Context, name string) (instance.State, error) {

	// parse config from name
	config, err := provider.convertName(name)
	if err != nil {
		return instance.ErrorInstanceState(name, err, config.Replicas)
	}

	// init Jobs service
	jobs := provider.Client.Jobs()

	job, _, err := jobs.Info(config.Job, &nomadapi.QueryOptions{})
	if err != nil {
		return instance.ErrorInstanceState(name, err, config.Replicas)
	}

	if job == nil {
		return instance.ErrorInstanceState(name, fmt.Errorf("could not find job"), config.Replicas)
	}

	if *job.Status == "dead" {
		return instance.NotReadyInstanceState(config.OriginalName, 0, config.Replicas)
	}

	for _, task := range job.TaskGroups {
		if *task.Name != config.Group {
			continue
		}
		currentReplicas := len(task.Tasks)
		// if currentReplicas >= config.Replicas && *job.Status == "running" {
		// 	return instance.ReadyInstanceState(config.OriginalName, currentReplicas)
		// }
		if currentReplicas != config.Replicas {
			return instance.NotReadyInstanceState(config.OriginalName, currentReplicas, config.Replicas)
		}
	}

	// init Deployments service
	deployments := provider.Client.Deployments()
	activeDeployments, _, err := deployments.List(&nomadapi.QueryOptions{
		Namespace: config.Namespace,
		Filter:    fmt.Sprintf("JobID == `%s` and JobVersion == `%d`", config.Job, *job.Version),
	})
	if err != nil {
		if serr, ok := err.(nomadapi.UnexpectedResponseError); ok {
			return instance.ErrorInstanceState(config.OriginalName, fmt.Errorf(serr.Body()), config.Replicas)
		}
		return instance.NotReadyInstanceState(config.OriginalName, 0, config.Replicas)
	}

	if len(activeDeployments) == 0 {
		return instance.NotReadyInstanceState(config.OriginalName, 0, config.Replicas)
	}

	if activeDeployments[0].Status == "successful" {
		return instance.ReadyInstanceState(config.OriginalName, config.Replicas)
	}

	if activeDeployments[0].Status == "failed" {
		return instance.ErrorInstanceState(config.OriginalName, fmt.Errorf(activeDeployments[0].StatusDescription), config.Replicas)
	}

	return instance.NotReadyInstanceState(config.OriginalName, 0, config.Replicas)
}

// GetGroups returns all jobs (the group) and the individual names of each task inside the group.
func (provider *NomadProvider) GetGroups(ctx context.Context) (map[string][]string, error) {
	return make(map[string][]string), nil
}

func (provider *NomadProvider) NotifyInstanceStopped(ctx context.Context, instance chan<- string) {

	events := provider.Client.EventStream()
	deployments := provider.Client.Deployments()

	q := &nomadapi.QueryOptions{}
	topics := map[nomadapi.Topic][]string{
		nomadapi.TopicJob: {"*"}, // subscribe to all Job updates
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	streamCh, err := events.Stream(ctx, topics, 0, q)
	if err != nil {
		log.Debug("could not open nomad events stream")
		return
	}

	for {
		select {
		case event := <-streamCh:
			if event.Err != nil {
				log.Debugf("provider event stream closed: %s", event.Err)
				return
			}

			for _, e := range event.Events {

				if e.Type != "EvaluationUpdated" {
					continue
				}

				// handle job evaluations
				if job, err := e.Job(); err == nil && job != nil {

					for _, taskgroup := range job.TaskGroups {
						if *job.Status != "dead" {
							continue
						}

						// get the previous deployment so we know what the replicas were
						deploymentList, _, err := deployments.List(&nomadapi.QueryOptions{
							Namespace: *job.Namespace,
							Filter:    fmt.Sprintf("JobID == `%s` and JobVersion == `%d`", *job.Name, *job.Version-1),
							PerPage:   1,
						})
						if err != nil || len(deploymentList) != 1 {
							continue
						}

						// notifiy that this instance has stopped
						instance <- fmt.Sprintf("%s@%s/%s/%d", *job.Name, *job.Namespace, *taskgroup.Name, deploymentList[0].TaskGroups[*taskgroup.Name].DesiredTotal)
					}
				}
			}

		case <-ctx.Done():
			return
		}
	}

}

// convertName parses the Name field from traefik into the target Namespace, Job and Group as "job@namespace/taskgroup/replicas"
// replicas defaults to 1; eg, "job@namespace/taskgroup" is valid
func (provider *NomadProvider) convertName(name string) (*NomadConfig, error) {
	config := NomadConfig{
		OriginalName: name,
		Replicas:     1,
	}

	// Split the first part based on '/'
	parts := strings.Split(name, "/")
	if len(parts) < 2 {
		return &config, errors.New("invalid name, should be: job@namespace/taskgroup/1")
	}

	config.Group = parts[1]

	// parts[0] contains "job@namespace" and parts[1] contains "taskgroup"
	subParts := strings.Split(parts[0], "@")
	if len(subParts) != 2 {
		return &config, errors.New("invalid name, should be: job@namespace/taskgroup/1")
	}
	config.Job = subParts[0]
	config.Namespace = subParts[1]

	// if replicas are defined, set them
	if len(parts) == 3 {
		var err error
		config.Replicas, err = strconv.Atoi(parts[2])
		if err != nil {
			return &config, errors.New("invalid name, error parsing replicas. should be: job@namespace/taskgroup/1")
		}
	}

	return &config, nil
}

func (provider *NomadProvider) scale(config *NomadConfig, replicas int) (instance.State, error) {
	// init Jobs service
	jobs := provider.Client.Jobs()

	// scale the service
	_, _, err := jobs.Scale(
		config.Job,
		config.Group,
		&replicas,
		fmt.Sprintf("Automatically scaled to %d from Sablier", replicas),
		false,
		make(map[string]interface{}),
		&nomadapi.WriteOptions{},
	)
	if err != nil {
		if serr, ok := err.(nomadapi.UnexpectedResponseError); ok {
			if serr.Body() == "job scaling blocked due to active deployment" {
				return instance.NotReadyInstanceState(config.OriginalName, 0, replicas)
			}
		}
		return instance.ErrorInstanceState(config.OriginalName, err, replicas)
	}

	return instance.NotReadyInstanceState(config.OriginalName, 0, replicas)
}
