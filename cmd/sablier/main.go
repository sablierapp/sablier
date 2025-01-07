package main

import (
	"context"
	"github.com/docker/docker/client"
	"github.com/rs/zerolog"
	"github.com/sablierapp/sablier/internal/server"
	"github.com/sablierapp/sablier/pkg/provider/docker"
	"github.com/sablierapp/sablier/pkg/sablier"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	zerolog.SetGlobalLevel(zerolog.TraceLevel)
	zerolog.DurationFieldInteger = true
	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).
		With().Timestamp().Caller().
		Logger()

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}

	p, err := docker.NewDockerProvider(cli, logger)
	if err != nil {
		panic(err)
	}

	s := sablier.NewSablier(ctx, p, logger)

	server.Start(ctx, s, logger)
}
