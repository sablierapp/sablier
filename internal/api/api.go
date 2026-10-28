package api

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
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
	Groups() map[string][]string
}

type ServeStrategy struct {
	Theme *theme.Themes

	Sablier        Sablier
	Metrics        metrics.Recorder
	StrategyConfig config.Strategy
	SessionsConfig config.Sessions

	// Leading reports whether this replica is the leader. Nil means leader
	// election is disabled and the replica always serves.
	Leading func() bool
}

// IsLeading reports whether this replica should serve session requests.
func (s *ServeStrategy) IsLeading() bool {
	return s.Leading == nil || s.Leading()
}

// RequireLeader rejects requests with 503 while this replica is a follower,
// so a request that reaches it anyway (e.g. during a failover, or through a
// Service that does not select the leader label) never acts on workloads
// behind the leader's back.
func RequireLeader(s *ServeStrategy) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !s.IsLeading() {
			AbortWithProblemDetail(c, ProblemNotLeader())
			c.Abort()
			return
		}
		c.Next()
	}
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
