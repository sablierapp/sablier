package dockerswarm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"

	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
)

func (p *Provider) NotifyInstanceStopped(ctx context.Context, instance chan<- string) {
	args := []filters.KeyValuePair{
		filters.Arg("scope", "swarm"),
		filters.Arg("type", "service"),
	}
	msgs, errs := p.Client.Events(ctx, events.ListOptions{
		Filters: filters.NewArgs(args...),
	})

	go func() {
		managedServices := map[string]struct{}{}
		if p.ignoreUnlabeled {
			services, err := p.managedServices(ctx)
			if err != nil {
				p.l.ErrorContext(ctx, "cannot list managed services", slog.Any("error", err))
				return
			}
			managedServices = services
		}

		for {
			select {
			case msg, ok := <-msgs:
				if !ok {
					p.l.ErrorContext(ctx, "event stream closed")
					return
				}
				p.l.DebugContext(ctx, "event received", "event", msg)

				name := msg.Actor.Attributes["name"]
				stopped := msg.Actor.Attributes["replicas.new"] == "0"
				removed := msg.Action == "remove"
				if !stopped && !removed {
					continue
				}

				if p.ignoreUnlabeled && !p.isManagedServiceEvent(ctx, managedServices, msg) {
					continue
				}

				instance <- name
				if removed {
					delete(managedServices, msg.Actor.ID)
					delete(managedServices, name)
				}
			case err, ok := <-errs:
				if !ok {
					p.l.ErrorContext(ctx, "event stream closed")
					return
				}
				if errors.Is(err, io.EOF) {
					p.l.ErrorContext(ctx, "event stream closed")
					return
				}
				p.l.ErrorContext(ctx, "event stream error", slog.Any("error", err))
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (p *Provider) managedServices(ctx context.Context) (map[string]struct{}, error) {
	args := filters.NewArgs()
	args.Add("label", fmt.Sprintf("%s=true", "sablier.enable"))

	services, err := p.Client.ServiceList(ctx, swarm.ServiceListOptions{Filters: args})
	if err != nil {
		return nil, fmt.Errorf("cannot list services: %w", err)
	}

	managed := make(map[string]struct{}, len(services)*2)
	for _, service := range services {
		managed[service.ID] = struct{}{}
		managed[service.Spec.Name] = struct{}{}
	}
	return managed, nil
}

func (p *Provider) isManagedServiceEvent(ctx context.Context, managed map[string]struct{}, msg events.Message) bool {
	if msg.Action == "remove" {
		return isKnownManagedService(managed, msg)
	}

	service, _, err := p.Client.ServiceInspectWithRaw(ctx, msg.Actor.ID, swarm.ServiceInspectOptions{})
	if err != nil {
		p.l.DebugContext(ctx, "cannot inspect service for label check", slog.String("service", msg.Actor.ID), slog.Any("error", err))
		return isKnownManagedService(managed, msg)
	}

	if service.Spec.Labels["sablier.enable"] == "true" {
		managed[service.ID] = struct{}{}
		managed[service.Spec.Name] = struct{}{}
		return true
	}

	delete(managed, service.ID)
	delete(managed, service.Spec.Name)
	return false
}

func isKnownManagedService(managed map[string]struct{}, msg events.Message) bool {
	if _, ok := managed[msg.Actor.ID]; ok {
		return true
	}
	if _, ok := managed[msg.Actor.Attributes["name"]]; ok {
		return true
	}
	return false
}
