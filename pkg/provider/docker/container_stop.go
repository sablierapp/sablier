package docker

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
)

func (p *Provider) waitForContainerToBePaused(ctx context.Context, name string) (<-chan container.WaitResponse, <-chan error) {
	waitC := make(chan container.WaitResponse)
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

func modifyMessageForPausing(message string, isPausing bool) string {
	if isPausing {
		message = strings.ReplaceAll(message, "stopp", "paus")
		message = strings.ReplaceAll(message, "stop", "pause")
	}
	return message
}

func (p *Provider) InstanceStop(ctx context.Context, name string) error {
	pauseInsteadOfStop := false
	containers, inspectErr := p.Client.ContainerInspect(ctx, name)
	if inspectErr == nil {
		pauseInsteadOfStop, _ = strconv.ParseBool(containers.Config.Labels["sablier.pauseOnly"])
		pauseInsteadOfStop = pauseInsteadOfStop && containers.State.Running
	}

	p.l.DebugContext(ctx, modifyMessageForPausing("stopping container", pauseInsteadOfStop), slog.String("name", name))
	var err error = nil
	if pauseInsteadOfStop {
		err = p.Client.ContainerPause(ctx, name)
	} else {
		err = p.Client.ContainerStop(ctx, name, container.StopOptions{})
	}

	if err != nil {
		p.l.ErrorContext(ctx, modifyMessageForPausing("cannot stop container", pauseInsteadOfStop), slog.String("name", name), slog.Any("error", err))
		return fmt.Errorf(modifyMessageForPausing("cannot stop container %s: %w", pauseInsteadOfStop), name, err)
	}

	p.l.DebugContext(ctx, modifyMessageForPausing("waiting for container to stop", pauseInsteadOfStop), slog.String("name", name))
	var waitC <-chan container.WaitResponse = nil
	var errC <-chan error = nil
	if pauseInsteadOfStop {
		waitC, errC = p.waitForContainerToBePaused(ctx, name)
	} else {
		waitC, errC = p.Client.ContainerWait(ctx, name, container.WaitConditionNotRunning)
	}
	select {
	case <-waitC:
		p.l.DebugContext(ctx, modifyMessageForPausing("container stopped", pauseInsteadOfStop), slog.String("name", name))
		return nil
	case err := <-errC:
		p.l.ErrorContext(ctx, modifyMessageForPausing("cannot wait for container to stop", pauseInsteadOfStop), slog.String("name", name), slog.Any("error", err))
		return fmt.Errorf(modifyMessageForPausing("cannot wait for container %s to stop: %w", pauseInsteadOfStop), name, err)
	case <-ctx.Done():
		p.l.ErrorContext(ctx, modifyMessageForPausing("context cancelled while waiting for container to stop", pauseInsteadOfStop), slog.String("name", name))
		return ctx.Err()
	}
}
