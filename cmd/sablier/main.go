package main

import (
	"context"
	"github.com/rs/zerolog"
	"github.com/sablierapp/sablier/internal/server"
	"github.com/sablierapp/sablier/pkg/sablier"
	"os/signal"
	"syscall"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	zerolog.SetGlobalLevel(zerolog.TraceLevel)

	s := sablier.NewSablier(ctx, nil)

	server.Start(ctx, s)
}
