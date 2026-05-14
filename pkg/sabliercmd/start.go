package sabliercmd

import (
	"context"
	"fmt"
	"log/slog"
	"os/signal"
	"syscall"
	"time"

	"github.com/sablierapp/sablier/internal/api"
	"github.com/sablierapp/sablier/internal/server"
	"github.com/sablierapp/sablier/pkg/config"
	"github.com/sablierapp/sablier/pkg/metrics"
	provpkg "github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
	"github.com/sablierapp/sablier/pkg/store/inmemory"
	"github.com/sablierapp/sablier/pkg/version"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var newStartCommand = func() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start the Sablier server",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := viper.Unmarshal(&conf); err != nil {
				return fmt.Errorf("unable to read configuration file: %w", err)
			}

			return Start(cmd.Context(), conf)
		},
	}
}

// Start starts the Sablier server
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

	rec := buildRecorder(conf.Server.Metrics.Enabled)
	store := inmemory.NewInMemory()
	s := sablier.New(logger, store, provider)
	s.WithMetrics(rec)
	s.WithRejectUnlabeledRequests(conf.Provider.RejectUnlabeledRequests)
	s.WithVerifyEnabledOnExpiration(conf.Provider.VerifyEnabledOnExpiration)
	err = store.OnExpire(ctx, s.OnInstanceExpired(ctx))
	if err != nil {
		return err
	}
	// Register the GroupLockCollector on the same registry so the gauges show
	// up alongside everything else when /metrics is scraped.
	if pr, ok := rec.(*metrics.PromRecorder); ok {
		pr.Registry().MustRegister(metrics.NewGroupLockCollector(s, pr))
	}
	s.BlockingRefreshFrequency = conf.Strategy.Blocking.DefaultRefreshFrequency

	groups, err := provider.InstanceGroups(ctx)
	if err != nil {
		logger.WarnContext(ctx, "initial group scan failed", slog.Any("reason", err))
	} else {
		s.SetGroups(groups)
	}

	go s.GroupWatch(ctx)
	stream := provider.InstanceEvents(ctx, provpkg.InstanceEventsOptions{
		Types: []provpkg.InstanceEventType{provpkg.InstanceEventStopped},
	})
	go func() {
		for {
			select {
			case event, ok := <-stream.Events:
				if !ok {
					return
				}
				err := s.RemoveInstance(ctx, event.Info.Name)
				if err != nil {
					logger.Warn("could not remove instance", slog.Any("error", err))
				}
			case err, ok := <-stream.Err:
				if !ok {
					return
				}
				logger.ErrorContext(ctx, "instance stopped event stream permanently lost", slog.Any("error", err))
				return
			}
		}
	}()

	if conf.Provider.AutoStopOnStartup {
		err := s.StopAllUnregisteredInstances(ctx)
		if err != nil {
			logger.ErrorContext(ctx, "unable to stop unregistered instances", slog.Any("reason", err))
		}
	}

	if conf.Provider.AutoStopExternallyStarted {
		go s.WatchAndStopExternallyStarted(ctx)
	}
	go s.WatchRunningHours(ctx)

	t, err := setupTheme(ctx, conf, logger)
	if err != nil {
		return fmt.Errorf("cannot setup theme: %w", err)
	}

	strategy := &api.ServeStrategy{
		Theme:          t,
		Sablier:        s,
		Metrics:        rec,
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

func buildRecorder(enabled bool) metrics.Recorder {
	if enabled {
		return metrics.NewPromRecorder()
	}
	return metrics.Noop{}
}
