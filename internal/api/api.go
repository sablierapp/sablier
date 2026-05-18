package api

import (
	"context"
	"time"

	"github.com/sablierapp/sablier/pkg/config"
	"github.com/sablierapp/sablier/pkg/metrics"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
	"github.com/sablierapp/sablier/pkg/theme"
)

//go:generate go tool -modfile=../../tools.mod mockgen -package apitest -source=api.go -destination=apitest/mocks_sablier.go *

type Sablier interface {
	RequestSession(ctx context.Context, names []string, duration time.Duration) (*sablier.SessionState, error)
	RequestSessionGroup(ctx context.Context, group string, duration time.Duration) (*sablier.SessionState, error)
	RequestReadySession(ctx context.Context, names []string, duration time.Duration, timeout time.Duration) (*sablier.SessionState, error)
	RequestReadySessionGroup(ctx context.Context, group string, duration time.Duration, timeout time.Duration) (*sablier.SessionState, error)
	InstanceEvents(ctx context.Context, opts provider.InstanceEventsOptions) sablier.InstanceEventStream
}

type ServeStrategy struct {
	Theme *theme.Themes

	Sablier        Sablier
	Metrics        metrics.Recorder
	StrategyConfig config.Strategy
	SessionsConfig config.Sessions
	ProviderConfig config.Provider
}

// recordSessionRequest emits the session-request counter for the given strategy
// based on whether the request targets named instances or a group.
func recordSessionRequest(rec metrics.Recorder, strategy, group string) {
	target := "names"
	if group != "" {
		target = "group"
	}
	rec.RecordSessionRequest(strategy, target)
}
