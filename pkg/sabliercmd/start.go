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
	"github.com/sablierapp/sablier/pkg/tracing"
	"github.com/sablierapp/sablier/pkg/version"
	"github.com/sablierapp/sablier/pkg/webhook"
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

	// Initialise OpenTelemetry tracing. The returned shutdown function flushes
	// all in-flight spans; it must be called before the process exits.
	tracingShutdown, err := tracing.Setup(ctx, conf.Tracing, logger)
	if err != nil {
		return fmt.Errorf("cannot setup tracing: %w", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := tracingShutdown(shutdownCtx); err != nil {
			logger.ErrorContext(ctx, "tracing shutdown error", slog.Any("error", err))
		}
	}()
	if conf.Tracing.Enabled {
		logger.Info("OpenTelemetry tracing enabled",
			slog.String("exporter", conf.Tracing.ExporterType),
			slog.String("endpoint", conf.Tracing.Endpoint),
			slog.String("service_name", conf.Tracing.ServiceName),
			slog.Float64("sampling_rate", conf.Tracing.SamplingRate),
		)
	}

	provider, err := setupProvider(ctx, logger, conf.Provider)
	if err != nil {
		return fmt.Errorf("cannot setup provider: %w", err)
	}

	rec := buildRecorder(conf.Server.Metrics.Enabled)
	store, save := setupStorage(ctx, logger, conf)

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
		pr.Registry().MustRegister(metrics.NewSessionExpiryCollector(s))
	}
	s.BlockingRefreshFrequency = conf.Strategy.Blocking.DefaultRefreshFrequency
	s.DefaultSessionDuration = conf.Sessions.DefaultDuration

	groups, err := provider.InstanceGroups(ctx)
	if err != nil {
		logger.WarnContext(ctx, "initial group scan failed", slog.Any("reason", err))
	} else {
		s.SetGroups(groups)
	}
	// Seed the anti-affinity index from pre-existing instances before watching.
	s.SeedAntiAffinity(ctx)

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
	if conf.Provider.AutoWarmExternallyStarted {
		go s.WatchAndWarmExternallyStarted(ctx)
	}
	go s.WatchRunningHours(ctx)

	if len(conf.Webhooks.Endpoints) > 0 {
		d := webhook.NewDispatcher(conf.Webhooks.Endpoints, logger)
		stream := s.InstanceEvents(ctx, provpkg.InstanceEventsOptions{
			Types: []provpkg.InstanceEventType{
				provpkg.InstanceEventStarted,
				provpkg.InstanceEventStopped,
			},
		})
		go d.Watch(ctx, stream)
		for i, ep := range conf.Webhooks.Endpoints {
			events := ep.Events
			if len(events) == 0 {
				events = []string{"started", "stopped"}
			}
			logger.InfoContext(ctx, "webhook endpoint registered",
				slog.Int("index", i),
				slog.String("url", ep.URL),
				slog.Any("events", events),
			)
			logger.DebugContext(ctx, "webhook endpoint configuration",
				slog.Int("index", i),
				slog.String("url", ep.URL),
				slog.Any("events", events),
				slog.Any("headers", ep.Headers),
			)
		}
	} else {
		logger.InfoContext(ctx, "no webhook endpoints configured")
	}

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

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.Start(ctx, logger, conf.Server, conf.Tracing, strategy)
	}()

	// Block until the process is signalled or the HTTP server dies. A serve
	// failure (e.g. the port is already in use) must terminate the process:
	// otherwise the watchers keep running with no listener and the instance
	// looks healthy while serving nothing.
	select {
	case <-ctx.Done():
		stop()
		logger.Info("shutting down gracefully, press Ctrl+C again to force")
		// Wait for the server to finish draining in-flight requests before
		// persisting state and exiting.
		err = <-serverErr
	case err = <-serverErr:
		stop()
	}

	save()

	logger.Info("Server exiting")

	return err
}

func buildRecorder(enabled bool) metrics.Recorder {
	if enabled {
		return metrics.NewPromRecorder()
	}
	return metrics.Noop{}
}
