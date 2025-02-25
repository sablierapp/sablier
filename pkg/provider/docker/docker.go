package docker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/sablierapp/sablier/app/discovery"
	"github.com/sablierapp/sablier/app/instance"
	"github.com/sablierapp/sablier/config"
	"github.com/sablierapp/sablier/pkg/provider"
)

// Interface guard
var _ provider.Provider = (*DockerClassicProvider)(nil)

type DockerClassicProvider struct {
	Client                client.APIClient
	desiredReplicas       int32
	reconnectInitialDelay time.Duration
	reconnectMaxDelay     time.Duration
	reconnectMaxAttempts  int
	l                     *slog.Logger
}

func NewDockerClassicProvider(ctx context.Context, logger *slog.Logger, dockerConfig config.Docker) (*DockerClassicProvider, error) {
	logger = logger.With(slog.String("provider", "docker"))
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("cannot create docker client: %v", err)
	}

	serverVersion, err := cli.ServerVersion(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to docker host: %v", err)
	}

	logger.InfoContext(ctx, "connection established with docker",
		slog.String("version", serverVersion.Version),
		slog.String("api_version", serverVersion.APIVersion),
		slog.Duration("reconnect_initial_delay", dockerConfig.ReconnectInitialDelay),
		slog.Duration("reconnect_max_delay", dockerConfig.ReconnectMaxDelay),
		slog.Int("reconnect_max_attempts", dockerConfig.ReconnectMaxAttempts),
	)
	return &DockerClassicProvider{
		Client:                cli,
		desiredReplicas:       1,
		reconnectInitialDelay: dockerConfig.ReconnectInitialDelay,
		reconnectMaxDelay:     dockerConfig.ReconnectMaxDelay,
		reconnectMaxAttempts:  dockerConfig.ReconnectMaxAttempts,
		l:                     logger,
	}, nil
}

func (p *DockerClassicProvider) GetGroups(ctx context.Context) (map[string][]string, error) {
	args := filters.NewArgs()
	args.Add("label", fmt.Sprintf("%s=true", discovery.LabelEnable))

	containers, err := p.Client.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: args,
	})

	if err != nil {
		return nil, err
	}

	groups := make(map[string][]string)
	for _, c := range containers {
		groupName := c.Labels[discovery.LabelGroup]
		if len(groupName) == 0 {
			groupName = discovery.LabelGroupDefaultValue
		}
		group := groups[groupName]
		group = append(group, strings.TrimPrefix(c.Names[0], "/"))
		groups[groupName] = group
	}

	return groups, nil
}

func (p *DockerClassicProvider) Start(ctx context.Context, name string) error {
	return p.Client.ContainerStart(ctx, name, container.StartOptions{})
}

func (p *DockerClassicProvider) Stop(ctx context.Context, name string) error {
	return p.Client.ContainerStop(ctx, name, container.StopOptions{})
}

func (p *DockerClassicProvider) GetState(ctx context.Context, name string) (instance.State, error) {
	spec, err := p.Client.ContainerInspect(ctx, name)
	if err != nil {
		return instance.State{}, err
	}

	// "created", "running", "paused", "restarting", "removing", "exited", or "dead"
	switch spec.State.Status {
	case "created", "paused", "restarting", "removing":
		return instance.NotReadyInstanceState(name, 0, p.desiredReplicas), nil
	case "running":
		if spec.State.Health != nil {
			// // "starting", "healthy" or "unhealthy"
			if spec.State.Health.Status == "healthy" {
				return instance.ReadyInstanceState(name, p.desiredReplicas), nil
			} else if spec.State.Health.Status == "unhealthy" {
				if len(spec.State.Health.Log) >= 1 {
					lastLog := spec.State.Health.Log[len(spec.State.Health.Log)-1]
					return instance.UnrecoverableInstanceState(name, fmt.Sprintf("container is unhealthy: %s (%d)", lastLog.Output, lastLog.ExitCode), p.desiredReplicas), nil
				} else {
					return instance.UnrecoverableInstanceState(name, "container is unhealthy: no log available", p.desiredReplicas), nil
				}
			} else {
				return instance.NotReadyInstanceState(name, 0, p.desiredReplicas), nil
			}
		}
		p.l.WarnContext(ctx, "container running without healthcheck, you should define a healthcheck on your container so that Sablier properly detects when the container is ready to handle requests.", slog.String("container", name))
		return instance.ReadyInstanceState(name, p.desiredReplicas), nil
	case "exited":
		if spec.State.ExitCode != 0 {
			return instance.UnrecoverableInstanceState(name, fmt.Sprintf("container exited with code \"%d\"", spec.State.ExitCode), p.desiredReplicas), nil
		}
		return instance.NotReadyInstanceState(name, 0, p.desiredReplicas), nil
	case "dead":
		return instance.UnrecoverableInstanceState(name, "container in \"dead\" state cannot be restarted", p.desiredReplicas), nil
	default:
		return instance.UnrecoverableInstanceState(name, fmt.Sprintf("container status \"%s\" not handled", spec.State.Status), p.desiredReplicas), nil
	}
}

func (p *DockerClassicProvider) NotifyInstanceStopped(ctx context.Context, instance chan<- string) {
	reconnectDelay := p.reconnectInitialDelay
	maxReconnectDelay := p.reconnectMaxDelay
	reconnect := true
	attempts := 0
	// Track if we are in reconnection mode to log successes
	reconnectionMode := false

	for reconnect {
		// Check if we've exceeded maximum reconnection attempts
		if p.reconnectMaxAttempts > 0 && attempts >= p.reconnectMaxAttempts {
			p.l.ErrorContext(ctx, "exceeded maximum reconnection attempts, giving up",
				slog.Int("max_attempts", p.reconnectMaxAttempts))
			return
		}

		select {
		case <-ctx.Done():
			return
		default:
			msgs, errs := p.Client.Events(ctx, types.EventsOptions{
				Filters: filters.NewArgs(
					filters.Arg("scope", "local"),
					filters.Arg("type", string(events.ContainerEventType)),
					filters.Arg("event", "die"),
				),
			})

			streamClosed := false
			for !streamClosed {
				select {
				case msg, ok := <-msgs:
					if !ok {
						p.l.WarnContext(ctx, "event stream closed, attempting to reconnect")
						streamClosed = true
						reconnectionMode = true
						continue
					}
					// Send the container that has died to the channel
					instance <- strings.TrimPrefix(msg.Actor.Attributes["name"], "/")
				case err, ok := <-errs:
					if !ok {
						p.l.WarnContext(ctx, "event stream closed, attempting to reconnect")
						streamClosed = true
						reconnectionMode = true
						continue
					}
					if errors.Is(err, io.EOF) {
						p.l.WarnContext(ctx, "event stream closed (EOF), attempting to reconnect")
						streamClosed = true
						reconnectionMode = true
						continue
					}
					p.l.ErrorContext(ctx, "event stream error", slog.Any("error", err))
					reconnectionMode = true
					// For other errors, we'll continue listening but log the error
				case <-ctx.Done():
					return
				}
			}

			// Verify Docker connectivity before reconnecting
			pingResult, err := p.Client.Ping(ctx)
			if err != nil {
				attempts++
				p.l.ErrorContext(ctx, "connection to Docker lost, will retry",
					slog.Any("error", err),
					slog.Duration("next_attempt", reconnectDelay),
					slog.Int("attempt", attempts),
					slog.Int("max_attempts", p.reconnectMaxAttempts))

				// Implement exponential backoff
				select {
				case <-time.After(reconnectDelay):
					// Double the delay for next attempt, up to the maximum
					reconnectDelay *= 2
					if reconnectDelay > maxReconnectDelay {
						reconnectDelay = maxReconnectDelay
					}
				case <-ctx.Done():
					return
				}
			} else {
				// If ping succeeds and we were in reconnection mode
				if reconnectionMode {
					p.l.InfoContext(ctx, "successfully reconnected to Docker",
						slog.String("status", "healthy"),
						slog.String("api_version", pingResult.APIVersion),
						slog.Int("attempts", attempts))

					// Reset counters and flags after successful reconnection
					attempts = 0
					reconnectionMode = false
				}
				// Always reset reconnect delay on successful ping
				reconnectDelay = p.reconnectInitialDelay
			}
		}
	}
}
