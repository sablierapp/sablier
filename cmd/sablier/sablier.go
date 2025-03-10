package main

import (
	"context"
	"fmt"
	"github.com/sablierapp/sablier/internal/api"
	"github.com/sablierapp/sablier/pkg/config"
	"github.com/sablierapp/sablier/pkg/sablier"
	"github.com/sablierapp/sablier/pkg/store/inmemory"
	"github.com/sablierapp/sablier/pkg/version"
	"log/slog"
	"os/signal"
	"syscall"
	"time"

	"github.com/sablierapp/sablier/internal/server"
)

func Start(ctx context.Context, conf config.Config) error {
	// Create context that listens for the interrupt signal from the OS.
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	logger := setupLogger(conf.Logging)

	logger.Info("running Sablier version " + version.Info())

	provider, err := setupProvider(ctx, logger, conf.Provider)
	if err != nil {
		return fmt.Errorf("cannot setup provider: %w", err)
	}

	store := inmemory.NewInMemory()
	err = store.OnExpire(ctx, sablier.OnInstanceExpired(ctx, provider, logger))
	if err != nil {
		return err
	}

	s := sablier.New(logger, store, provider)

	groups, err := provider.InstanceGroups(ctx)
	if err != nil {
		logger.WarnContext(ctx, "initial group scan failed", slog.Any("reason", err))
	} else {
		s.SetGroups(groups)
	}

	go s.GroupWatch(ctx)
	instanceStopped := make(chan string)
	go provider.NotifyInstanceStopped(ctx, instanceStopped)
	go func() {
		for stopped := range instanceStopped {
			err := s.RemoveInstance(ctx, stopped)
			if err != nil {
				logger.Warn("could not remove instance", slog.Any("error", err))
			}
		}
	}()

	if conf.Provider.AutoStopOnStartup {
		err := s.StopAllUnregisteredInstances(ctx)
		if err != nil {
			logger.ErrorContext(ctx, "unable to stop unregistered instances", slog.Any("reason", err))
		}
	}

	t, err := setupTheme(ctx, conf, logger)
	if err != nil {
		return fmt.Errorf("cannot setup theme: %w", err)
	}

	strategy := &api.ServeStrategy{
		Theme:          t,
		Sablier:        s,
		StrategyConfig: conf.Strategy,
		SessionsConfig: conf.Sessions,
	}

	go server.Start(ctx, logger, conf.Server, strategy)

	// Listen for the interrupt signal.
	<-ctx.Done()

	stop()
	logger.InfoContext(ctx, "shutting down gracefully, press Ctrl+C again to force")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	logger.InfoContext(ctx, "Server exiting")

	return nil
}
