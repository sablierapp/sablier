package docker

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/docker/docker/api/types/container"
)

func (p *Provider) waitForContainerToBePaused(ctx context.Context, name string) (chan struct{}, chan error) {
	waitC := make(chan struct{})
	errC := make(chan error, 1)

	go func() {
		defer close(waitC)

		for {
			containerInspect, err := p.Client.ContainerInspect(ctx, name)
			if err != nil {
				errC <- fmt.Errorf("could not inspect container: %w", err)
				return
			}

			if containerInspect.State.Paused {
				close(waitC)
				return
			}

			time.Sleep(200 * time.Millisecond)
		}
	}()

	return waitC, errC
}

func (p *Provider) InstanceStop(ctx context.Context, name string) error {
	pauseInsteadOfStop := false
	containers, inspectErr := p.Client.ContainerInspect(ctx, name)
	if inspectErr == nil {
		pauseInsteadOfStop, _ = strconv.ParseBool(containers.Config.Labels["sablier.pauseOnly"])
	}

	if pauseInsteadOfStop && containers.State.Running {
		p.l.DebugContext(ctx, "pausing container", slog.String("name", name))
		err := p.Client.ContainerPause(ctx, name)
		if err != nil {
			p.l.ErrorContext(ctx, "cannot pause container", slog.String("name", name), slog.Any("error", err))
			return fmt.Errorf("cannot pause container %s: %w", name, err)
		}

		p.l.DebugContext(ctx, "waiting for container to pause", slog.String("name", name))
		waitC, errC := p.waitForContainerToBePaused(ctx, name)
		select {
		case <-waitC:
			p.l.DebugContext(ctx, "container paused", slog.String("name", name))
			return nil
		case err := <-errC:
			p.l.ErrorContext(ctx, "cannot wait for container to pause", slog.String("name", name), slog.Any("error", err))
			return fmt.Errorf("cannot wait for container %s to pause: %w", name, err)
		case <-ctx.Done():
			p.l.ErrorContext(ctx, "context cancelled while waiting for container to pause", slog.String("name", name))
			return ctx.Err()
		}
	} else {
		p.l.DebugContext(ctx, "stopping container", slog.String("name", name))
		err := p.Client.ContainerStop(ctx, name, container.StopOptions{})
		if err != nil {
			p.l.ErrorContext(ctx, "cannot stop container", slog.String("name", name), slog.Any("error", err))
			return fmt.Errorf("cannot stop container %s: %w", name, err)
		}

		p.l.DebugContext(ctx, "waiting for container to stop", slog.String("name", name))
		waitC, errC := p.Client.ContainerWait(ctx, name, container.WaitConditionNotRunning)
		select {
		case <-waitC:
			p.l.DebugContext(ctx, "container stopped", slog.String("name", name))
			return nil
		case err := <-errC:
			p.l.ErrorContext(ctx, "cannot wait for container to stop", slog.String("name", name), slog.Any("error", err))
			return fmt.Errorf("cannot wait for container %s to stop: %w", name, err)
		case <-ctx.Done():
			p.l.ErrorContext(ctx, "context cancelled while waiting for container to stop", slog.String("name", name))
			return ctx.Err()
		}
	}
}
