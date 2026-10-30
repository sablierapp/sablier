package sabliercmd

import (
	"context"
	"fmt"
	"log/slog"
	"os/signal"
	"sync/atomic"
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
		logger.Info(
			"OpenTelemetry tracing enabled",
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
	// Register the GroupLockCollector on the same registry so the gauges show
	// up alongside everything else when /metrics is scraped.
	if pr, ok := rec.(*metrics.PromRecorder); ok {
		pr.Registry().MustRegister(metrics.NewGroupLockCollector(s, pr))
		pr.Registry().MustRegister(metrics.NewSessionExpiryCollector(s))
	}
	s.BlockingRefreshFrequency = conf.Strategy.Blocking.DefaultRefreshFrequency
	s.DefaultSessionDuration = conf.Sessions.DefaultDuration

	if conf.Activity.Enabled {
		src, closer, err := setupActivity(conf.Activity, conf.Provider.Name, provider, logger)
		if err != nil {
			return err
		}
		watching := make(chan struct{})
		go func() {
			defer close(watching)
			s.WatchActivity(ctx, src, conf.Activity.Interval)
		}()
		// Stop the loop before unloading the eBPF programs it reads.
		defer func() {
			stop()
			<-watching
			if err := closer.Close(); err != nil {
				logger.ErrorContext(ctx, "cannot unload the activity eBPF programs", slog.Any("error", err))
			}
		}()
		logger.InfoContext(ctx, "activity tracking enabled",
			slog.Duration("interval", conf.Activity.Interval),
			slog.Any("ignore_cidrs", conf.Activity.IgnoreCIDRs),
		)
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

	electionConf := conf.Provider.Kubernetes.LeaderElection
	// leading is only consulted when leader election is enabled.
	var leading, everLed atomic.Bool
	// lost is closed when leadership is lost, electionErr receives a failure
	// to set up the election or to activate after winning it.
	lost := make(chan struct{})
	electionErr := make(chan error, 1)
	// Only the first election failure matters; never block on reporting one.
	electionFailed := func(err error) {
		select {
		case electionErr <- err:
		default:
		}
	}
	var election *leaderElection
	if electionConf.Enabled {
		strategy.Leading = leading.Load
		election, err = newLeaderElection(ctx, logger, electionConf)
		if err != nil {
			return err
		}
	} else if err := activate(ctx, logger, conf, s, store, provider, false); err != nil {
		return err
	}

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.Start(ctx, logger, conf.Server, conf.Tracing, strategy)
	}()

	// electionDone is closed once the elector has returned, after releasing
	// the Lease on shutdown.
	electionDone := make(chan struct{})
	if election == nil {
		close(electionDone)
	} else {
		go func() {
			defer close(electionDone)
			err := election.run(ctx, func(leaderCtx context.Context) {
				if err := activate(leaderCtx, logger, conf, s, store, provider, true); err != nil {
					electionFailed(fmt.Errorf("cannot activate leader: %w", err))
					return
				}
				leading.Store(true)
				everLed.Store(true)
				go func() {
					<-leaderCtx.Done()
					leading.Store(false)
				}()
				// Only now route the Service to this pod.
				if err := election.setLeaderLabel(leaderCtx, true); err != nil {
					electionFailed(err)
				}
			}, lost)
			if err != nil {
				electionFailed(err)
			}
		}()
	}

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
	case <-lost:
		stop()
		<-serverErr
		err = errLeadershipLost
	case err = <-electionErr:
		stop()
		<-serverErr
	}

	// Wait for the Lease release, otherwise the process may exit first and
	// the next leader has to wait for the Lease to expire.
	<-electionDone

	// A follower's store is empty: saving it would overwrite the state file
	// the leader persisted.
	if !electionConf.Enabled || everLed.Load() {
		save()
	}

	logger.Info("Server exiting")

	return err
}

func buildRecorder(enabled bool) metrics.Recorder {
	if enabled {
		return metrics.NewPromRecorder()
	}
	return metrics.Noop{}
}

// activate starts everything that acts on workloads: session expiration, the
// group and instance watchers, auto-stop/auto-warm and webhooks. All of it
// stops when ctx is cancelled. With leader election it runs on the leader
// only, and takeover is true.
func activate(ctx context.Context, logger *slog.Logger, conf config.Config, s *sablier.Sablier, store sablier.Store, provider sablier.Provider, takeover bool) error {
	err := store.OnExpire(ctx, s.OnInstanceExpired(ctx))
	if err != nil {
		return err
	}

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

	switch {
	case takeover:
		// A new leader cannot tell a fresh start from a failover, and its
		// store is empty either way. Stopping unregistered instances would
		// stop workloads that still have sessions on the previous leader, so
		// give them a default-duration session instead and let them expire.
		go func() {
			if err := s.WarmAllUnregisteredInstances(ctx); err != nil {
				logger.ErrorContext(ctx, "unable to warm running instances on takeover", slog.Any("reason", err))
			}
		}()
	case conf.Provider.AutoStopOnStartup:
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
			logger.InfoContext(
				ctx, "webhook endpoint registered",
				slog.Int("index", i),
				slog.String("url", ep.URL),
				slog.Any("events", events),
			)
			logger.DebugContext(
				ctx, "webhook endpoint configuration",
				slog.Int("index", i),
				slog.String("url", ep.URL),
				slog.Any("events", events),
				slog.Any("headers", ep.Headers),
			)
		}
	} else {
		logger.InfoContext(ctx, "no webhook endpoints configured")
	}

	return nil
}
