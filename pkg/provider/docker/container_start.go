package docker

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/moby/moby/client"
	"github.com/sablierapp/sablier/pkg/sablier"
)

func (p *Provider) InstanceStart(ctx context.Context, name string) error {
	spec, err := p.Client.ContainerInspect(ctx, name, client.ContainerInspectOptions{})
	if err != nil {
		return fmt.Errorf("cannot inspect container: %w", err)
	}

	sc := sablier.ScaleConfigFromLabels(spec.Container.Config.Labels)
	if sc != nil && (sc.Active.CPU != "" || sc.Active.Memory != "") {
		p.l.DebugContext(ctx, "applying active resources (scale mode)",
			slog.String("name", name),
			slog.String("cpu", sc.Active.CPU),
			slog.String("memory", sc.Active.Memory),
		)
		return p.applyResources(ctx, name, sc.Active.CPU, sc.Active.Memory)
	}

	if p.strategy == "pause" {
		return p.dockerUnpause(ctx, name)
	}
	return p.dockerStart(ctx, name)
}

func (p *Provider) dockerStart(ctx context.Context, name string) error {
	// TODO: InstanceStart should block until the container is ready.
	p.l.DebugContext(ctx, "starting container", slog.String("name", name))
	_, err := p.Client.ContainerStart(ctx, name, client.ContainerStartOptions{})
	if err != nil {
		p.l.ErrorContext(ctx, "cannot start container", slog.String("name", name), slog.Any("error", err))
		return fmt.Errorf("cannot start container %s: %w", name, err)
	}
	return nil
}

func (p *Provider) dockerUnpause(ctx context.Context, name string) error {
	inspected, inspectErr := p.Client.ContainerInspect(ctx, name, client.ContainerInspectOptions{})
	if inspectErr != nil {
		p.l.ErrorContext(ctx, "cannot inspect container before unpausing", slog.String("name", name), slog.Any("error", inspectErr))
		return fmt.Errorf("cannot inspect container %s before unpausing: %w", name, inspectErr)
	}

	if !inspected.Container.State.Paused {
		p.l.DebugContext(ctx, "container is not paused, starting container", slog.String("name", name))
		return p.dockerStart(ctx, name)
	}

	p.l.DebugContext(ctx, "unpausing container", slog.String("name", name))
	_, err := p.Client.ContainerUnpause(ctx, name, client.ContainerUnpauseOptions{})
	if err != nil {
		p.l.ErrorContext(ctx, "cannot unpause container", slog.String("name", name), slog.Any("error", err))
		return fmt.Errorf("cannot unpause container %s: %w", name, err)
	}

	p.l.DebugContext(ctx, "container unpaused", slog.String("name", name))
	return nil
}
