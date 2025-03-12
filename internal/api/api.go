package api

import (
	"context"
	"github.com/sablierapp/sablier/pkg/config"
	"github.com/sablierapp/sablier/pkg/sablier"
	"github.com/sablierapp/sablier/pkg/theme"
	"time"
)

//go:generate go tool mockgen -package apitest -source=api.go -destination=apitest/mocks_sablier.go *

type Sablier interface {
	RequestSession(ctx context.Context, names []string, duration time.Duration) (*sablier.SessionState, error)
	RequestSessionGroup(ctx context.Context, group string, duration time.Duration) (*sablier.SessionState, error)
	RequestReadySession(ctx context.Context, names []string, duration time.Duration, timeout time.Duration) (*sablier.SessionState, error)
	RequestReadySessionGroup(ctx context.Context, group string, duration time.Duration, timeout time.Duration) (*sablier.SessionState, error)
}

type ServeStrategy struct {
	Theme *theme.Themes

	Sablier        Sablier
	StrategyConfig config.Strategy
	SessionsConfig config.Sessions
}
